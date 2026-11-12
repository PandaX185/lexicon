set shell := ["bash", "-uc"]

preset := env("PRESET", "dev-debug")

default:
    @just --list

configure:
    cmake --preset {{ preset }}

build: configure
    cmake --build --preset {{ preset }}

run: build
    ./build/{{ preset }}/lexicon

test: build
    ctest --preset {{ preset }}

test-all: build
    ctest --test-dir build/{{ preset }} --output-on-failure

test-kde: build
    ctest --test-dir build/{{ preset }} -L kde --output-on-failure

format:
    find src tests -type f \( -name '*.cpp' -o -name '*.h' \) -print0 | xargs -0 clang-format -i

format-check:
    find src tests -type f \( -name '*.cpp' -o -name '*.h' \) -print0 | xargs -0 clang-format --dry-run --Werror

qml-lint: configure
    cmake --build --preset {{ preset }} --target qml-lint

lint:
    cmake --preset lint
    cmake --build --preset lint
    find src tests -type f -name '*.cpp' -print0 | xargs -0 clang-tidy -p build/lint --quiet --warnings-as-errors='*'

check: format-check qml-lint test

coverage:
    cmake --preset coverage
    cmake --build --preset coverage
    ctest --preset coverage
    cmake -E make_directory build/coverage-report
    gcovr --root . \
        --filter 'src/' \
        --exclude 'third_party/' \
        --exclude-unreachable-branches \
        --exclude-throw-branches \
        --print-summary \
        --xml-pretty --output build/coverage-report/coverage.xml \
        --html-details build/coverage-report/index.html

seed-db:
    mkdir -p build/dev
    rm -f build/dev/lexicon.sqlite
    sqlite3 build/dev/lexicon.sqlite < data/dictionaries/schema.sql
    sqlite3 build/dev/lexicon.sqlite < data/dictionaries/dev-seed.sql

demo: build seed-db
    QT_QPA_PLATFORM=offscreen ./build/{{ preset }}/lexicon --lookup serendipity --database build/dev/lexicon.sqlite

install-check: build
    rm -rf build/install-root
    DESTDIR="$PWD/build/install-root" cmake --install build/{{ preset }} --prefix /usr
    ./scripts/check-install.sh build/install-root

clean:
    cmake -E remove_directory build

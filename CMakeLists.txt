cmake_minimum_required(VERSION 3.24)

project(Lexicon
    VERSION 0.1.0
    DESCRIPTION "Lightweight offline English dictionary utility for KDE Plasma"
    HOMEPAGE_URL "https://github.com/PandaX185/lexicon"
    LANGUAGES C CXX
)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(LEXICON_APP_ID "org.lexicon.Lexicon")
set(LEXICON_COMPONENT_NAME "lexicon")

find_package(ECM 6.0 REQUIRED NO_MODULE)
set(CMAKE_MODULE_PATH ${ECM_MODULE_PATH} ${CMAKE_MODULE_PATH})
include(KDEInstallDirs6)
include(KDECompilerSettings NO_POLICY_SCOPE)
include(ECMEnableSanitizers)

find_package(Qt6 6.10 REQUIRED COMPONENTS
    Core
    Gui
    Widgets
    Qml
    Quick
    QuickControls2
    Sql
    Test
    DBus
    Concurrent
)
find_package(KF6 6.0 REQUIRED COMPONENTS
    GlobalAccel
    StatusNotifierItem
    DBusAddons
)

find_package(tomlplusplus 3.4 QUIET)
if(tomlplusplus_FOUND)
    set(LEXICON_TOML_TARGET tomlplusplus::tomlplusplus)
    message(STATUS "Using system tomlplusplus ${tomlplusplus_VERSION}")
else()
    add_library(lexicon_tomlplusplus INTERFACE)
    target_include_directories(lexicon_tomlplusplus SYSTEM INTERFACE ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    set(LEXICON_TOML_TARGET lexicon_tomlplusplus)
    message(STATUS "Using vendored toml++ 3.4.0 (MIT)")
endif()

qt_standard_project_setup(REQUIRES 6.10)
enable_testing()

option(LEXICON_ENABLE_SANITIZERS "Build with AddressSanitizer and UndefinedBehaviorSanitizer" OFF)
option(LEXICON_WARNINGS_AS_ERRORS "Treat compiler warnings as errors" OFF)
option(LEXICON_ENABLE_COVERAGE "Build with GCC/Clang coverage instrumentation" OFF)

# Platform tiering: the application depends only on the SelectionProvider and
# PrimarySelectionSource interfaces, so any backend can be omitted. A build
# without a backend still runs and reports an explicit unsupported error.
option(LEXICON_WITH_WAYLAND "Build the Wayland primary-selection backend" ON)

if(LEXICON_ENABLE_COVERAGE)
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        add_compile_options(--coverage -O0 -g)
        add_link_options(--coverage)
    else()
        message(FATAL_ERROR "Coverage is only supported with GCC or Clang")
    endif()
endif()

if(LEXICON_ENABLE_SANITIZERS)
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
        add_link_options(-fsanitize=address,undefined)
    else()
        message(FATAL_ERROR "Sanitizers are only supported with GCC or Clang")
    endif()
endif()

function(lexicon_enable_warnings target)
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        target_compile_options(${target} PRIVATE
            -Wall
            -Wextra
            -Wpedantic
            -Wshadow
            -Wnon-virtual-dtor
            -Wold-style-cast
            -Wcast-align
            -Wunused
        )
        if(LEXICON_WARNINGS_AS_ERRORS)
            target_compile_options(${target} PRIVATE -Werror)
        endif()
    endif()
endfunction()

add_subdirectory(src)

set(LEXICON_QML_FILES
    ui/Main.qml
    ui/popup/LookupPopup.qml
    ui/settings/SettingsView.qml
    ui/components/DefinitionList.qml
    ui/components/SectionCard.qml
    ui/themes/Theme.qml
)

qt_add_resources(lexicon "lexicon_resources"
    PREFIX "/"
    FILES
        ${LEXICON_QML_FILES}
        resources/icons/lexicon.svg
)

if(TARGET Qt6::qmllint)
    add_custom_target(qml-lint
        COMMAND Qt6::qmllint ${LEXICON_QML_FILES}
        WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}
        COMMENT "Linting QML sources"
        VERBATIM
    )
else()
    message(WARNING "Qt6::qmllint is not available; the qml-lint target will not be created")
endif()

add_subdirectory(tests)

include(CTest)

install(TARGETS lexicon RUNTIME DESTINATION ${KDE_INSTALL_BINDIR})
install(FILES packaging/shared/${LEXICON_APP_ID}.desktop DESTINATION ${KDE_INSTALL_APPDIR})
install(FILES resources/icons/lexicon.svg DESTINATION ${KDE_INSTALL_ICONDIR}/hicolor/scalable/apps)
install(FILES packaging/shared/${LEXICON_APP_ID}.metainfo.xml DESTINATION ${KDE_INSTALL_METAINFODIR})
install(FILES resources/config/config.example.toml DESTINATION ${KDE_INSTALL_DATADIR}/lexicon)
install(FILES data/dictionaries/schema.sql DESTINATION ${KDE_INSTALL_DATADIR}/lexicon/dictionaries)
install(FILES packaging/shared/lexicon-autostart.desktop
        DESTINATION ${KDE_INSTALL_DATADIR}/lexicon)
install(FILES README.md LICENSE CHANGELOG.md ROADMAP.md CONTRIBUTING.md SECURITY.md SUPPORT.md
    DESTINATION ${KDE_INSTALL_DATADIR}/doc/lexicon
)
install(DIRECTORY docs/ DESTINATION ${KDE_INSTALL_DATADIR}/doc/lexicon/docs)

# Offline English Dictionary Resources and Downloadable Argos Translation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement bundled offline English WordNet dictionary resources, CMUdict-derived offline IPA pronunciation data, and an optional downloadable Argos Translate language-pair system with a static catalog and local Python worker subprocess for Lexicon.

**Architecture:** Extend Lexicon's existing multi-dictionary catalog to search multi-tier XDG directories (bundled `/usr/share/lexicon` and user `~/.local/share/lexicon`), bundle a WordNet 3.0 SQLite dictionary with pre-converted IPA pronunciation, and manage offline Argos translation via a lazy `QProcess` Python worker using a newline-delimited JSON protocol over stdin/stdout paired with a `QNetworkAccessManager` static catalog downloader.

**Tech Stack:** C++20, Qt 6 (Core, Network, Sql, Qml, Quick, Concurrent), KDE Frameworks 6 (KGlobalAccel, KStatusNotifierItem), SQLite 3, Python 3.10+ (Argos Translate, WordNet preprocessor), TOML (toml++).

**Spec:** Documented in [docs/dictionary-format.md](file:///data/Projects/lexicon/docs/dictionary-format.md), [docs/configuration.md](file:///data/Projects/lexicon/docs/configuration.md), and [docs/architecture/overview.md](file:///data/Projects/lexicon/docs/architecture/overview.md).

## Global Constraints

- Lexicon architecture and inward-only dependencies (`src/core/` -> `src/dictionary/` -> `src/infrastructure/` -> `src/app/`) must remain strictly intact.
- SQLite repository connection isolation: worker threads must never share `QSqlDatabase` connections; short-lived read-only connections are created per query.
- No network requests on startup or during dictionary lookup; dictionary content and pronunciation must be 100% offline.
- Translation worker must run as a local subprocess via `QProcess` communicating strictly via JSON over stdin/stdout; no HTTP, TCP, gRPC, or daemon servers.
- Stdin/stdout are reserved strictly for protocol JSON; stderr is used for diagnostics.
- Models must never be downloaded automatically or implicitly; all model installations require explicit user initiation.
- Language pack archives must be verified via SHA-256 before installation into the Argos model repository.
- Bundled resources in system directories must never be mutated or unnecessarily copied into user-writable directories.
- Existing CMake options, including `LEXICON_WITH_WAYLAND`, must remain supported.
- Code style must adhere to `.clang-format` (LLVM base, 4-space indent, column limit 120, pointer left-aligned) and `.clang-tidy`.

---

### Task 1: Multi-Directory Dictionary Catalog Resolution

**Files:**
- Modify: `src/dictionary/DictionaryCatalog.h:15-47`
- Modify: `src/dictionary/DictionaryCatalog.cpp:11-40`
- Modify: `src/config/Config.h:82-85`
- Modify: `src/config/Config.cpp:74-85`
- Modify: `src/app/Application.cpp:32-45`
- Test: `tests/unit/test_dictionary_catalog.cpp`

**Interfaces:**
- Consumes:
  - `Lexicon::DictionaryInfo` from `src/dictionary/models/DictionaryInfo.h`
  - `Lexicon::DictionaryRepository` from `src/dictionary/DictionaryRepository.h`
- Produces:
  - `DictionaryCatalog::DictionaryCatalog(const QStringList& directories, RepositoryFactory factory)`
  - `DictionaryCatalog::directories() const -> QStringList`
  - `Config::resolvedDictionaryDirectories() const -> QStringList`

- [ ] **Step 1: Write failing unit test for multi-directory catalog resolution**

Add test cases in `tests/unit/test_dictionary_catalog.cpp`:

```cpp
void DictionaryCatalogTest::scansMultipleDirectoriesWithPriority() {
    QTemporaryDir systemDir;
    QTemporaryDir userDir;
    QVERIFY(systemDir.isValid());
    QVERIFY(userDir.isValid());

    writeFile(systemDir, QStringLiteral("english.sqlite"));
    writeFile(userDir, QStringLiteral("english.sqlite")); // User override
    writeFile(systemDir, QStringLiteral("german.sqlite"));

    DictionaryInfos systemInfos{
        {QStringLiteral("english"), info("System English", "en", "en")},
        {QStringLiteral("german"), info("German", "de", "de")},
    };
    DictionaryInfos userInfos{
        {QStringLiteral("english"), info("User English", "en", "en")},
    };

    auto factory = [systemInfos, userInfos](const QString& path) -> std::unique_ptr<DictionaryRepository> {
        const QString base = QFileInfo(path).completeBaseName();
        if (path.contains(QStringLiteral("user")) || path.contains(QStringLiteral("tmp"))) {
            if (userInfos.contains(base)) {
                return std::make_unique<StubRepository>(path, userInfos);
            }
        }
        return std::make_unique<StubRepository>(path, systemInfos);
    };

    const DictionaryCatalog catalog(QStringList{userDir.path(), systemDir.path()}, factory);

    QCOMPARE(catalog.entries().size(), 2);
    const auto englishEntry = catalog.find(QStringLiteral("en"), QStringLiteral("en"));
    QVERIFY(englishEntry.has_value());
    QCOMPARE(englishEntry->info.name, QStringLiteral("User English"));
    QVERIFY(englishEntry->path.startsWith(userDir.path()));

    const auto germanEntry = catalog.find(QStringLiteral("de"), QStringLiteral("de"));
    QVERIFY(germanEntry.has_value());
    QCOMPARE(germanEntry->info.name, QStringLiteral("German"));
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `ctest --test-dir build/dev-debug -R test_dictionary_catalog --output-on-failure`
Expected: FAIL (compilation failure: constructor taking `QStringList` does not exist).

- [ ] **Step 3: Implement multi-directory catalog scanning and configuration resolution**

In `src/dictionary/DictionaryCatalog.h`:
```cpp
class DictionaryCatalog {
  public:
    using RepositoryFactory = std::function<std::unique_ptr<DictionaryRepository>(const QString& path)>;

    explicit DictionaryCatalog(QString directory, RepositoryFactory factory = {});
    explicit DictionaryCatalog(QStringList directories, RepositoryFactory factory = {});

    void refresh();

    [[nodiscard]] const QList<DictionaryCatalogEntry>& entries() const noexcept;
    [[nodiscard]] QString directory() const;
    [[nodiscard]] QStringList directories() const;
    [[nodiscard]] std::optional<DictionaryCatalogEntry> find(const QString& sourceLanguage,
                                                             const QString& targetLanguage) const;

  private:
    QStringList m_directories;
    RepositoryFactory m_factory;
    QList<DictionaryCatalogEntry> m_entries;
};
```

In `src/dictionary/DictionaryCatalog.cpp`:
```cpp
DictionaryCatalog::DictionaryCatalog(QString directory, RepositoryFactory factory)
    : DictionaryCatalog(QStringList{std::move(directory)}, std::move(factory)) {}

DictionaryCatalog::DictionaryCatalog(QStringList directories, RepositoryFactory factory)
    : m_directories(std::move(directories)), m_factory(std::move(factory)) {
    refresh();
}

void DictionaryCatalog::refresh() {
    QList<DictionaryCatalogEntry> entries;
    QSet<QString> seenFileNames;
    if (m_factory) {
        for (const QString& dirPath : m_directories) {
            const QDir directory(dirPath);
            const QFileInfoList files =
                directory.entryInfoList({QStringLiteral("*.sqlite")}, QDir::Files | QDir::Readable, QDir::Name);
            for (const QFileInfo& file : files) {
                const QString fileName = file.fileName();
                if (seenFileNames.contains(fileName)) {
                    continue; // Earlier directory in list takes precedence
                }
                const QString path = file.absoluteFilePath();
                const std::unique_ptr<DictionaryRepository> repository = m_factory(path);
                if (!repository) {
                    continue;
                }
                const DictionaryDescription description = repository->describe();
                if (!description.ok() || !description.info.isValid()) {
                    continue;
                }
                seenFileNames.insert(fileName);
                entries.append(DictionaryCatalogEntry{
                    .info = description.info,
                    .path = path,
                    .entryCount = description.entryCount,
                });
            }
        }
    }
    m_entries = std::move(entries);
}

QStringList DictionaryCatalog::directories() const {
    return m_directories;
}

QString DictionaryCatalog::directory() const {
    return m_directories.isEmpty() ? QString() : m_directories.constFirst();
}
```

In `src/config/Config.h` and `src/config/Config.cpp`:
```cpp
QStringList Config::resolvedDictionaryDirectories() const {
    if (!dictionary.directory.trimmed().isEmpty()) {
        return {dictionary.directory};
    }
    QStringList dirs;
    const QString subPath = QStringLiteral("/lexicon/dictionaries");
    const QStringList dataDirs = QStandardPaths::standardLocations(QStandardPaths::GenericDataLocation);
    for (const QString& base : dataDirs) {
        dirs.append(base + subPath);
    }
    return dirs;
}
```

In `src/app/Application.cpp`:
```cpp
std::unique_ptr<Lexicon::DictionaryRepository> buildDictionaryRepository(const Lexicon::Config& config) {
    const auto open = [](const QString& path) -> std::unique_ptr<Lexicon::DictionaryRepository> {
        return std::make_unique<Lexicon::SqliteDictionaryRepository>(path);
    };

    if (!config.usesDictionaryDirectory()) {
        return open(config.resolvedDatabasePath());
    }

    return std::make_unique<Lexicon::CatalogDictionaryRepository>(
        Lexicon::DictionaryCatalog(config.resolvedDictionaryDirectories(), open), open, config.dictionary.sourceLanguage,
        config.dictionary.targetLanguage);
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_dictionary_catalog --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add src/dictionary/DictionaryCatalog.h src/dictionary/DictionaryCatalog.cpp src/config/Config.h src/config/Config.cpp src/app/Application.cpp tests/unit/test_dictionary_catalog.cpp
git commit -m "feat(dictionary): support multi-tier catalog directory discovery"
```

---

### Task 2: IPA Pronunciation Style Configuration & UI Presenter

**Files:**
- Modify: `src/config/Config.h:31-34,97`
- Modify: `src/config/Config.cpp:49-55,125`
- Modify: `src/config/ConfigLoader.cpp:338-347`
- Modify: `src/app/QmlPresenter.cpp:22-54`
- Modify: `resources/config/config.example.toml:41-44`
- Test: `tests/unit/test_config.cpp`

**Interfaces:**
- Consumes:
  - `PronunciationStyle` in `src/config/Config.h`
  - `WordEntry` from `src/dictionary/models/WordEntry.h`
- Produces:
  - `PronunciationStyle::Ipa`
  - `pronunciationStyleToString(PronunciationStyle::Ipa) -> "ipa"`
  - `entryToVariant` passing `pronunciation.ipa`, `pronunciation.respelling`, and `pronunciation.text` (selected according to style)

- [ ] **Step 1: Write failing test for IPA pronunciation configuration**

In `tests/unit/test_config.cpp`:

```cpp
void ConfigLoaderTest::loadsIpaPronunciationStyle() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[dictionary]
show_pronunciation = true
pronunciation_style = "ipa"
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(result.found);
    QVERIFY(!result.usedDefaults);
    QVERIFY(result.issues.isEmpty());
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Ipa);
    QCOMPARE(pronunciationStyleToString(result.config.dictionary.pronunciationStyle), QStringLiteral("ipa"));
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `ctest --test-dir build/dev-debug -R test_config --output-on-failure`
Expected: FAIL (compilation failure: `PronunciationStyle::Ipa` does not exist).

- [ ] **Step 3: Implement PronunciationStyle::Ipa enum and presentation mapping**

In `src/config/Config.h`:
```cpp
enum class PronunciationStyle {
    Respelling,
    Ipa,
};
```

In `src/config/Config.cpp`:
```cpp
QString pronunciationStyleToString(PronunciationStyle value) {
    switch (value) {
    case PronunciationStyle::Respelling:
        return QStringLiteral("respelling");
    case PronunciationStyle::Ipa:
        return QStringLiteral("ipa");
    }
    return QStringLiteral("respelling");
}
```

In `src/config/ConfigLoader.cpp`:
```cpp
QString style;
if (readString(dictionary, "pronunciation_style", style, result, QStringLiteral("dictionary"))) {
    if (style == QStringLiteral("respelling")) {
        result.config.dictionary.pronunciationStyle = PronunciationStyle::Respelling;
    } else if (style == QStringLiteral("ipa")) {
        result.config.dictionary.pronunciationStyle = PronunciationStyle::Ipa;
    } else {
        addWarning(result, QStringLiteral("dictionary.pronunciation_style"),
                   QStringLiteral("Unsupported pronunciation style; keeping respelling."));
    }
}
```

In `src/app/QmlPresenter.cpp`:
```cpp
QVariantMap entryToVariant(const Lexicon::WordEntry& entry, bool showPronunciation,
                           Lexicon::PronunciationStyle style) {
    QVariantMap result;
    result.insert(QStringLiteral("word"), entry.word);
    result.insert(QStringLiteral("transliteration"), entry.transliteration);

    QVariantMap pronunciation;
    pronunciation.insert(QStringLiteral("respelling"), entry.pronunciation.respelling);
    pronunciation.insert(QStringLiteral("ipa"), entry.pronunciation.ipa);

    QString formattedText;
    if (style == Lexicon::PronunciationStyle::Ipa) {
        formattedText = !entry.pronunciation.ipa.isEmpty() ? entry.pronunciation.ipa
                                                           : entry.pronunciation.respelling;
    } else {
        formattedText = !entry.pronunciation.respelling.isEmpty() ? entry.pronunciation.respelling
                                                                 : entry.pronunciation.ipa;
    }
    pronunciation.insert(QStringLiteral("text"), formattedText);
    pronunciation.insert(QStringLiteral("style"), pronunciationStyleToString(style));
    result.insert(QStringLiteral("pronunciation"), pronunciation);
    ...
```

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_config --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add src/config/Config.h src/config/Config.cpp src/config/ConfigLoader.cpp src/app/QmlPresenter.cpp resources/config/config.example.toml tests/unit/test_config.cpp
git commit -m "feat(config): support IPA pronunciation style"
```

---

### Task 3: ARPABET-to-IPA Conversion Pipeline

**Files:**
- Create: `scripts/lexicon_importer/arpabet_to_ipa.py`
- Create: `src/dictionary/ArpabetToIpa.h`
- Create: `src/dictionary/ArpabetToIpa.cpp`
- Modify: `src/dictionary/CMakeLists.txt`
- Create: `tests/unit/test_arpabet_to_ipa.cpp`
- Modify: `tests/CMakeLists.txt`

**Interfaces:**
- Consumes: Raw ARPABET phoneme strings with stress markers (e.g., `"S EH2 R AH0 N D IH1 P IH0 T IY0"`)
- Produces:
  - Python: `arpabet_to_ipa(arpabet_str: str) -> str`
  - C++: `Lexicon::arpabetToIpa(const QString& arpabet) -> QString`

- [ ] **Step 1: Write failing C++ unit test for ARPABET-to-IPA conversion**

Create `tests/unit/test_arpabet_to_ipa.cpp`:

```cpp
#include "dictionary/ArpabetToIpa.h"
#include <QTest>

using namespace Lexicon;

class ArpabetToIpaTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void convertsSingleWord();
    void handlesPrimaryAndSecondaryStress();
    void handlesUnstressedVowels();
    void handlesConsonantsAndAffricates();
    void handlesMultiplePronunciationVariants();
    void handlesInvalidInputGracefully();
};

void ArpabetToIpaTest::convertsSingleWord() {
    // serendipity: S EH2 R AH0 N D IH1 P IH0 T IY0 -> /ˌsɛrənˈdɪpɪti/
    const QString arpabet = QStringLiteral("S EH2 R AH0 N D IH1 P IH0 T IY0");
    const QString ipa = arpabetToIpa(arpabet);
    QCOMPARE(ipa, QStringLiteral("/ˌsɛrənˈdɪpɪti/"));
}

void ArpabetToIpaTest::handlesPrimaryAndSecondaryStress() {
    // lexicon: L EH1 K S IH0 K AA2 N -> /ˈlɛksɪkˌɑn/
    const QString arpabet = QStringLiteral("L EH1 K S IH0 K AA2 N");
    const QString ipa = arpabetToIpa(arpabet);
    QCOMPARE(ipa, QStringLiteral("/ˈlɛksɪkˌɑn/"));
}

void ArpabetToIpaTest::handlesUnstressedVowels() {
    // apple: AE1 P AH0 L -> /ˈæpəl/
    QCOMPARE(arpabetToIpa(QStringLiteral("AE1 P AH0 L")), QStringLiteral("/ˈæpəl/"));
}

void ArpabetToIpaTest::handlesConsonantsAndAffricates() {
    // church: CH ER1 CH -> /ˈtʃɜːrtʃ/
    QCOMPARE(arpabetToIpa(QStringLiteral("CH ER1 CH")), QStringLiteral("/ˈtʃɜːrtʃ/"));
    // judge: JH AH1 JH -> /ˈdʒʌdʒ/
    QCOMPARE(arpabetToIpa(QStringLiteral("JH AH1 JH")), QStringLiteral("/ˈdʒʌdʒ/"));
    // the: DH AH0 -> /ðə/
    QCOMPARE(arpabetToIpa(QStringLiteral("DH AH0")), QStringLiteral("/ðə/"));
    // think: TH IH1 NG K -> /ˈθɪŋk/
    QCOMPARE(arpabetToIpa(QStringLiteral("TH IH1 NG K")), QStringLiteral("/ˈθɪŋk/"));
}

void ArpabetToIpaTest::handlesMultiplePronunciationVariants() {
    // Variant delimiter handling
    const QString ipa = arpabetToIpa(QStringLiteral("R EH1 D ; R IY1 D"));
    QCOMPARE(ipa, QStringLiteral("/ˈrɛd/, /ˈriːd/"));
}

void ArpabetToIpaTest::handlesInvalidInputGracefully() {
    QCOMPARE(arpabetToIpa(QStringLiteral("")), QStringLiteral(""));
    QCOMPARE(arpabetToIpa(QStringLiteral("   ")), QStringLiteral(""));
}

QTEST_GUILESS_MAIN(ArpabetToIpaTest)
#include "test_arpabet_to_ipa.moc"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cmake --build --preset dev-debug --target test_arpabet_to_ipa`
Expected: FAIL (files missing).

- [ ] **Step 3: Implement ARPABET-to-IPA conversion in Python and C++**

Create `scripts/lexicon_importer/arpabet_to_ipa.py`:

```python
"""ARPABET to IPA conversion module for Lexicon."""

import re

PHONEME_MAP = {
    'AA': 'ɑ', 'AE': 'æ', 'AH': 'ʌ', 'AO': 'ɔ', 'AW': 'aʊ',
    'AY': 'aɪ', 'B': 'b', 'CH': 'tʃ', 'D': 'd', 'DH': 'ð',
    'EH': 'ɛ', 'ER': 'ɜːr', 'EY': 'eɪ', 'F': 'f', 'G': 'ɡ',
    'HH': 'h', 'IH': 'ɪ', 'IY': 'iː', 'JH': 'dʒ', 'K': 'k',
    'L': 'l', 'M': 'm', 'N': 'n', 'NG': 'ŋ', 'OW': 'oʊ',
    'OY': 'ɔɪ', 'P': 'p', 'R': 'r', 'S': 's', 'SH': 'ʃ',
    'T': 't', 'TH': 'θ', 'UH': 'ʊ', 'UW': 'uː', 'V': 'v',
    'W': 'w', 'Y': 'j', 'Z': 'z', 'ZH': 'ʒ'
}

VOWELS = {
    'AA', 'AE', 'AH', 'AO', 'AW', 'AY', 'EH', 'ER',
    'EY', 'IH', 'IY', 'OW', 'OY', 'UH', 'UW'
}

def single_arpabet_to_ipa(tokens: list[str]) -> str:
    if not tokens:
        return ""
    syllables = []
    current_consonants = []
    
    for token in tokens:
        m = re.match(r"^([A-Z]+)([0-2])?$", token)
        if not m:
            continue
        phoneme, stress = m.group(1), m.group(2)
        if phoneme in VOWELS:
            vowel_ipa = PHONEME_MAP.get(phoneme, phoneme)
            if phoneme == 'AH' and stress == '0':
                vowel_ipa = 'ə'
            elif phoneme == 'ER' and stress == '0':
                vowel_ipa = 'ər'
            
            stress_mark = ""
            if stress == '1':
                stress_mark = "ˈ"
            elif stress == '2':
                stress_mark = "ˌ"
            
            # Place stress before onset consonants of this syllable
            onset_ipa = "".join(PHONEME_MAP.get(c, c) for c in current_consonants)
            syllables.append(f"{stress_mark}{onset_ipa}{vowel_ipa}")
            current_consonants = []
        else:
            current_consonants.append(phoneme)
            
    coda_ipa = "".join(PHONEME_MAP.get(c, c) for c in current_consonants)
    return f"/{''.join(syllables)}{coda_ipa}/"

def arpabet_to_ipa(arpabet_str: str) -> str:
    cleaned = arpabet_str.strip()
    if not cleaned:
        return ""
    variants = [v.strip().split() for v in cleaned.split(";") if v.strip()]
    ipa_variants = [single_arpabet_to_ipa(v) for v in variants if v]
    return ", ".join(ipa_variants)
```

Create `src/dictionary/ArpabetToIpa.h`:

```cpp
#pragma once

#include <QString>

namespace Lexicon {

[[nodiscard]] QString arpabetToIpa(const QString& arpabet);

} // namespace Lexicon
```

Create `src/dictionary/ArpabetToIpa.cpp`:

```cpp
#include "dictionary/ArpabetToIpa.h"

#include <QHash>
#include <QRegularExpression>
#include <QStringList>

namespace Lexicon {
namespace {

const QHash<QString, QString>& phonemeMap() {
    static const QHash<QString, QString> map = {
        {QStringLiteral("AA"), QStringLiteral("ɑ")},   {QStringLiteral("AE"), QStringLiteral("æ")},
        {QStringLiteral("AH"), QStringLiteral("ʌ")},   {QStringLiteral("AO"), QStringLiteral("ɔ")},
        {QStringLiteral("AW"), QStringLiteral("aʊ")},  {QStringLiteral("AY"), QStringLiteral("aɪ")},
        {QStringLiteral("B"), QStringLiteral("b")},    {QStringLiteral("CH"), QStringLiteral("tʃ")},
        {QStringLiteral("D"), QStringLiteral("d")},    {QStringLiteral("DH"), QStringLiteral("ð")},
        {QStringLiteral("EH"), QStringLiteral("ɛ")},   {QStringLiteral("ER"), QStringLiteral("ɜːr")},
        {QStringLiteral("EY"), QStringLiteral("eɪ")},  {QStringLiteral("F"), QStringLiteral("f")},
        {QStringLiteral("G"), QStringLiteral("ɡ")},    {QStringLiteral("HH"), QStringLiteral("h")},
        {QStringLiteral("IH"), QStringLiteral("ɪ")},   {QStringLiteral("IY"), QStringLiteral("iː")},
        {QStringLiteral("JH"), QStringLiteral("dʒ")},  {QStringLiteral("K"), QStringLiteral("k")},
        {QStringLiteral("L"), QStringLiteral("l")},    {QStringLiteral("M"), QStringLiteral("m")},
        {QStringLiteral("N"), QStringLiteral("n")},    {QStringLiteral("NG"), QStringLiteral("ŋ")},
        {QStringLiteral("OW"), QStringLiteral("oʊ")},  {QStringLiteral("OY"), QStringLiteral("ɔɪ")},
        {QStringLiteral("P"), QStringLiteral("p")},    {QStringLiteral("R"), QStringLiteral("r")},
        {QStringLiteral("S"), QStringLiteral("s")},    {QStringLiteral("SH"), QStringLiteral("ʃ")},
        {QStringLiteral("T"), QStringLiteral("t")},    {QStringLiteral("TH"), QStringLiteral("θ")},
        {QStringLiteral("UH"), QStringLiteral("ʊ")},   {QStringLiteral("UW"), QStringLiteral("uː")},
        {QStringLiteral("V"), QStringLiteral("v")},    {QStringLiteral("W"), QStringLiteral("w")},
        {QStringLiteral("Y"), QStringLiteral("j")},    {QStringLiteral("Z"), QStringLiteral("z")},
        {QStringLiteral("ZH"), QStringLiteral("ʒ")},
    };
    return map;
}

const QSet<QString>& vowels() {
    static const QSet<QString> set = {
        QStringLiteral("AA"), QStringLiteral("AE"), QStringLiteral("AH"), QStringLiteral("AO"),
        QStringLiteral("AW"), QStringLiteral("AY"), QStringLiteral("EH"), QStringLiteral("ER"),
        QStringLiteral("EY"), QStringLiteral("IH"), QStringLiteral("IY"), QStringLiteral("OW"),
        QStringLiteral("OY"), QStringLiteral("UH"), QStringLiteral("UW"),
    };
    return set;
}

QString convertSingleVariant(const QStringList& tokens) {
    if (tokens.isEmpty()) {
        return {};
    }
    const auto& map = phonemeMap();
    const auto& vowelSet = vowels();
    static const QRegularExpression regex(QStringLiteral("^([A-Z]+)([0-2])?$"));

    QString result;
    QStringList currentConsonants;

    for (const QString& token : tokens) {
        const QRegularExpressionMatch match = regex.match(token);
        if (!match.hasMatch()) {
            continue;
        }
        const QString phoneme = match.captured(1);
        const QString stress = match.captured(2);

        if (vowelSet.contains(phoneme)) {
            QString vowelIpa = map.value(phoneme, phoneme);
            if (phoneme == QStringLiteral("AH") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("ə");
            } else if (phoneme == QStringLiteral("ER") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("ər");
            }

            QString stressMark;
            if (stress == QStringLiteral("1")) {
                stressMark = QStringLiteral("ˈ");
            } else if (stress == QStringLiteral("2")) {
                stressMark = QStringLiteral("ˌ");
            }

            result += stressMark;
            for (const QString& consonant : currentConsonants) {
                result += map.value(consonant, consonant);
            }
            currentConsonants.clear();
            result += vowelIpa;
        } else {
            currentConsonants.append(phoneme);
        }
    }

    for (const QString& consonant : currentConsonants) {
        result += map.value(consonant, consonant);
    }

    return QStringLiteral("/%1/").arg(result);
}

} // namespace

QString arpabetToIpa(const QString& arpabet) {
    const QString trimmed = arpabet.trimmed();
    if (trimmed.isEmpty()) {
        return {};
    }

    const QStringList rawVariants = trimmed.split(QLatin1Char(';'), Qt::SkipEmptyParts);
    QStringList convertedVariants;
    for (const QString& rawVariant : rawVariants) {
        const QStringList tokens = rawVariant.trimmed().split(QRegularExpression(QStringLiteral("\\s+")), Qt::SkipEmptyParts);
        const QString converted = convertSingleVariant(tokens);
        if (!converted.isEmpty()) {
            convertedVariants.append(converted);
        }
    }

    return convertedVariants.join(QStringLiteral(", "));
}

} // namespace Lexicon
```

Update `src/dictionary/CMakeLists.txt` to add `ArpabetToIpa.cpp` and `ArpabetToIpa.h`.
Update `tests/CMakeLists.txt` to add `test_arpabet_to_ipa`.

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_arpabet_to_ipa --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add scripts/lexicon_importer/arpabet_to_ipa.py src/dictionary/ArpabetToIpa.h src/dictionary/ArpabetToIpa.cpp src/dictionary/CMakeLists.txt tests/unit/test_arpabet_to_ipa.cpp tests/CMakeLists.txt
git commit -m "feat(dictionary): add ARPABET to IPA pronunciation converter"
```

---

### Task 4: WordNet 3.0 Dataset Importer & Bundled SQLite Generation

**Files:**
- Create: `scripts/lexicon_importer/import_wordnet.py`
- Create: `scripts/lexicon_importer/cmudict_parser.py`
- Create: `licenses/WORDNET.txt`
- Create: `licenses/CMUDICT.txt`
- Modify: `CMakeLists.txt:137-140`
- Modify: `justfile:50-55`
- Create: `tests/integration/test_wordnet_sqlite.cpp`

**Interfaces:**
- Consumes:
  - Princeton WordNet 3.0 data files (`data.noun`, `data.verb`, `data.adj`, `data.adv`, `index.*`)
  - CMUdict 0.7b text file
  - Lexicon SQLite schema `data/dictionaries/schema.sql` (user_version 2)
- Produces:
  - Bundled database: `data/dictionaries/en.sqlite`
  - Attributions in `dictionary_info` table:
    `name="WordNet English Dictionary"`, `source_language="en"`, `target_language="en"`, `license="WordNet 3.0"`, `attribution="WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved. Pronunciation data from CMU Pronouncing Dictionary."`

- [ ] **Step 1: Write integration test for WordNet SQLite database verification**

Create `tests/integration/test_wordnet_sqlite.cpp`:

```cpp
#include "infrastructure/database/SqliteDictionaryRepository.h"
#include <QFile>
#include <QTest>

using namespace Lexicon;

class WordNetSqliteTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void databaseExistsAndHasMetadata();
    void headwordHasDefinitionsAndIpa();
    void multipleSensesAndPosPreserved();
    void lookupIsFastUnderThreshold();
};

void WordNetSqliteTest::databaseExistsAndHasMetadata() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet in development tree.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryDescription desc = repo.describe();
    QVERIFY(desc.ok());
    QCOMPARE(desc.info.sourceLanguage, QStringLiteral("en"));
    QCOMPARE(desc.info.targetLanguage, QStringLiteral("en"));
    QVERIFY(desc.info.attribution.contains(QStringLiteral("Princeton University")));
    QVERIFY(desc.entryCount > 50000);
}

void WordNetSqliteTest::headwordHasDefinitionsAndIpa() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryLookupResult result = repo.findByWord(QStringLiteral("serendipity"));
    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("serendipity"));
    QVERIFY(!result.entry->definitions.isEmpty());
    QVERIFY(!result.entry->pronunciation.ipa.isEmpty());
    QVERIFY(result.entry->pronunciation.ipa.startsWith(QLatin1Char('/')));
}

void WordNetSqliteTest::multipleSensesAndPosPreserved() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryLookupResult result = repo.findByWord(QStringLiteral("run"));
    QVERIFY(result.found());
    QVERIFY(result.entry->definitions.size() > 5);

    bool hasVerb = false;
    bool hasNoun = false;
    for (const auto& def : result.entry->definitions) {
        if (def.partOfSpeech == PartOfSpeech::Verb) hasVerb = true;
        if (def.partOfSpeech == PartOfSpeech::Noun) hasNoun = true;
    }
    QVERIFY(hasVerb);
    QVERIFY(hasNoun);
}

void WordNetSqliteTest::lookupIsFastUnderThreshold() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    QBENCHMARK {
        const auto res = repo.findByWord(QStringLiteral("lexicon"));
        Q_UNUSED(res);
    }
}

QTEST_GUILESS_MAIN(WordNetSqliteTest)
#include "test_wordnet_sqlite.moc"
```

- [ ] **Step 2: Run test to verify it skips or fails**

Run: `cmake --build --preset dev-debug --target test_wordnet_sqlite && ctest --test-dir build/dev-debug -R test_wordnet_sqlite`
Expected: SKIPPED (database not generated yet).

- [ ] **Step 3: Implement Python WordNet 3.0 & CMUdict SQLite generator script**

Create `scripts/lexicon_importer/cmudict_parser.py`:

```python
"""Parser for CMUdict formatted phonemes."""

import re
from collections import defaultdict
from .arpabet_to_ipa import arpabet_to_ipa

def parse_cmudict(filepath: str) -> dict[str, str]:
    pronunciations = defaultdict(list)
    with open(filepath, 'r', encoding='latin-1') as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith(';;;'):
                continue
            parts = line.split()
            word_part = parts[0]
            phonemes = " ".join(parts[1:])
            # Clean word: e.g. "WORD(2)" -> "word"
            clean_word = re.sub(r'\(\d+\)$', '', word_part).lower()
            pronunciations[clean_word].append(phonemes)
            
    # Combine variants into single semicolon separated string and convert to IPA
    ipa_dict = {}
    for word, variant_list in pronunciations.items():
        combined_arpabet = " ; ".join(variant_list)
        ipa = arpabet_to_ipa(combined_arpabet)
        if ipa:
            ipa_dict[word] = ipa
    return ipa_dict
```

Create `scripts/lexicon_importer/import_wordnet.py`:

```python
#!/usr/bin/env python3
"""Builds Lexicon production SQLite database from Princeton WordNet and CMUdict."""

import argparse
import os
import re
import sqlite3
from .cmudict_parser import parse_cmudict

POS_MAP = {
    'n': 'noun',
    'v': 'verb',
    'a': 'adjective',
    's': 'adjective', # adjective satellite
    'r': 'adverb'
}

def parse_wordnet_data_file(filepath: str, pos_code: str):
    """Parses WordNet data.<pos> file and yields (synset_id, pos, words, gloss, examples)."""
    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith('  '):
                continue
            parts = line.split('|')
            if len(parts) < 2:
                continue
            header = parts[0].strip().split()
            gloss_part = parts[1].strip()
            
            # Extract examples in quotes
            examples = re.findall(r'"([^"]*)"', gloss_part)
            clean_def = re.sub(r';\s*"[^"]*"', '', gloss_part).strip(' ;"').capitalize()
            
            w_cnt = int(header[3], 16)
            words = []
            idx = 4
            for _ in range(w_cnt):
                word = header[idx].replace('_', ' ')
                words.append(word)
                idx += 2
                
            yield POS_MAP.get(pos_code, 'other'), words, clean_def, examples

def build_database(wordnet_dir: str, cmudict_file: str, output_sqlite: str, schema_file: str):
    if os.path.exists(output_sqlite):
        os.remove(output_sqlite)
        
    conn = sqlite3.connect(output_sqlite)
    cur = conn.cursor()
    
    with open(schema_file, 'r', encoding='utf-8') as sf:
        schema_sql = sf.read()
    cur.executescript(schema_sql)
    
    # Insert dictionary_info
    cur.execute("""
        INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
        VALUES (1, 'WordNet English Dictionary', 'en', 'en', 'WordNet 3.0',
                'WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved. Pronunciation data from CMU Pronouncing Dictionary.')
    """)
    
    # Load IPA pronunciations
    print("Loading CMUdict IPA pronunciations...")
    cmudict = parse_cmudict(cmudict_file) if cmudict_file and os.path.exists(cmudict_file) else {}
    
    # Process WordNet synsets
    print("Parsing WordNet synsets...")
    entries_map = {} # word -> { 'id': int, 'ipa': str, 'senses': list }
    word_id_counter = 1
    
    pos_files = [('noun', 'n'), ('verb', 'v'), ('adj', 'a'), ('adv', 'r')]
    for pos_name, pos_code in pos_files:
        data_path = os.path.join(wordnet_dir, f"data.{pos_name}")
        if not os.path.exists(data_path):
            continue
        for pos, words, definition, examples in parse_wordnet_data_file(data_path, pos_code):
            for word in words:
                norm_word = word.lower()
                if norm_word not in entries_map:
                    ipa = cmudict.get(norm_word, "")
                    entries_map[norm_word] = {
                        'id': word_id_counter,
                        'word': word,
                        'ipa': ipa,
                        'definitions': []
                    }
                    word_id_counter += 1
                
                label = ""
                full_def = definition
                if examples:
                    full_def += " (e.g. \"" + '"; "'.join(examples) + "\")"
                
                entries_map[norm_word]['definitions'].append((pos, full_def, label))
                
    print(f"Writing {len(entries_map)} headwords to SQLite...")
    cur.execute("BEGIN TRANSACTION")
    def_id_counter = 1
    for item in entries_map.values():
        cur.execute("""
            INSERT INTO entries (id, word, transliteration, pronunciation_respelling, pronunciation_ipa, source, license)
            VALUES (?, ?, '', '', ?, 'WordNet 3.0', 'WordNet 3.0')
        """, (item['id'], item['word'], item['ipa']))
        
        for sense_order, (pos, def_text, label) in enumerate(item['definitions']):
            cur.execute("""
                INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
                VALUES (?, ?, ?, ?, ?, ?)
            """, (def_id_counter, item['id'], pos, def_text, label, sense_order))
            def_id_counter += 1
            
    conn.commit()
    print("Optimizing SQLite database...")
    cur.execute("ANALYZE")
    cur.execute("VACUUM")
    conn.close()
    print(f"Successfully generated {output_sqlite}")

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--wordnet-dir', required=True)
    parser.add_argument('--cmudict-file', required=True)
    parser.add_argument('--schema', default='data/dictionaries/schema.sql')
    parser.add_argument('--output', default='data/dictionaries/en.sqlite')
    args = parser.parse_args()
    build_database(args.wordnet_dir, args.cmudict_file, args.output, args.schema)
```

Add license files `licenses/WORDNET.txt` and `licenses/CMUDICT.txt`.
Update `CMakeLists.txt` to install the bundled dictionary:
```cmake
install(FILES data/dictionaries/en.sqlite DESTINATION ${KDE_INSTALL_DATADIR}/lexicon/dictionaries)
install(FILES licenses/WORDNET.txt licenses/CMUDICT.txt DESTINATION ${KDE_INSTALL_DATADIR}/doc/lexicon/licenses)
```

- [ ] **Step 4: Run integration test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_wordnet_sqlite --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add scripts/lexicon_importer/ licenses/ CMakeLists.txt justfile tests/integration/test_wordnet_sqlite.cpp
git commit -m "feat(dictionary): add WordNet and CMUdict preprocessing pipeline and bundled en.sqlite"
```

---

### Task 5: Local Argos Translate Python Worker Process & Protocol

**Files:**
- Create: `scripts/argos_worker.py`
- Create: `src/infrastructure/translation/ArgosProcessWorker.h`
- Create: `src/infrastructure/translation/ArgosProcessWorker.cpp`
- Modify: `src/infrastructure/CMakeLists.txt`
- Create: `tests/unit/test_argos_worker_protocol.cpp`
- Modify: `tests/CMakeLists.txt`

**Interfaces:**
- Protocol: Newline-delimited JSON over stdin/stdout.
  - Stdin Request: `{"id": int, "version": 1, "action": string, "params": object}`
  - Stdout Response: `{"id": int, "version": 1, "success": bool, "result": object, "error": object}`
- C++ Class: `ArgosProcessWorker`
  - `start() -> ServiceStatus`
  - `stop()`
  - `sendRequest(const QString& action, const QJsonObject& params, int timeoutMs = 5000) -> Result<QJsonObject>`

- [ ] **Step 1: Write unit test for Argos worker JSON protocol and subprocess lifecycle**

Create `tests/unit/test_argos_worker_protocol.cpp`:

```cpp
#include "infrastructure/translation/ArgosProcessWorker.h"
#include <QJsonObject>
#include <QTest>

using namespace Lexicon;

class ArgosWorkerProtocolTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void workerStartsAndRepliesToPing();
    void reportsInstalledPairs();
    void handlesPairNotInstalledError();
    void handlesMalformedResponseGracefully();
    void handlesWorkerTimeout();
};

void ArgosWorkerProtocolTest::workerStartsAndRepliesToPing() {
    ArgosProcessWorker worker;
    const ServiceStatus status = worker.start();
    QVERIFY(status.available);

    const auto response = worker.sendRequest(QStringLiteral("ping"), {});
    QVERIFY(response.hasValue());
    QCOMPARE(response->value(QStringLiteral("status")).toString(), QStringLiteral("ok"));
    QCOMPARE(response->value(QStringLiteral("version")).toInt(), 1);
    worker.stop();
}

void ArgosWorkerProtocolTest::reportsInstalledPairs() {
    ArgosProcessWorker worker;
    worker.start();
    const auto response = worker.sendRequest(QStringLiteral("list_installed_pairs"), {});
    QVERIFY(response.hasValue());
    QVERIFY(response->value(QStringLiteral("pairs")).isArray());
    worker.stop();
}

void ArgosWorkerProtocolTest::handlesPairNotInstalledError() {
    ArgosProcessWorker worker;
    worker.start();
    QJsonObject params{
        {QStringLiteral("text"), QStringLiteral("hello")},
        {QStringLiteral("from"), QStringLiteral("en")},
        {QStringLiteral("to"), QStringLiteral("zz")},
    };
    const auto response = worker.sendRequest(QStringLiteral("translate"), params);
    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::DatabaseUnavailable);
    QVERIFY(response.error().message.contains(QStringLiteral("not installed")));
    worker.stop();
}

void ArgosWorkerProtocolTest::handlesMalformedResponseGracefully() {
    // Tests resilience against unparseable stdout
    ArgosProcessWorker worker(QStringLiteral("/bin/echo"), {QStringLiteral("NOT JSON")});
    worker.start();
    const auto response = worker.sendRequest(QStringLiteral("ping"), {});
    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::ConfigParseError);
    worker.stop();
}

void ArgosWorkerProtocolTest::handlesWorkerTimeout() {
    // Tests hung process kill
    ArgosProcessWorker worker(QStringLiteral("/bin/sleep"), {QStringLiteral("10")});
    worker.start();
    const auto response = worker.sendRequest(QStringLiteral("ping"), {}, 500); // 500ms timeout
    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::Unknown);
    worker.stop();
}

QTEST_GUILESS_MAIN(ArgosWorkerProtocolTest)
#include "test_argos_worker_protocol.moc"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cmake --build --preset dev-debug --target test_argos_worker_protocol`
Expected: FAIL (`ArgosProcessWorker` missing).

- [ ] **Step 3: Implement Python worker and C++ QProcess manager**

Create `scripts/argos_worker.py`:

```python
#!/usr/bin/env python3
"""Argos Translate local subprocess worker."""

import json
import sys

def get_argos():
    try:
        import argostranslate.package
        import argostranslate.translate
        return argostranslate
    except ImportError:
        return None

def handle_ping(params):
    return {"status": "ok", "version": 1}

def handle_list_installed_pairs(params):
    argos = get_argos()
    if not argos:
        return {"pairs": []}
    installed_languages = argos.translate.get_installed_languages()
    pairs = []
    for from_lang in installed_languages:
        for to_lang in from_lang.translations_from:
            pairs.append({
                "from": from_lang.code,
                "from_name": from_lang.name,
                "to": to_lang.to_lang.code,
                "to_name": to_lang.to_lang.name
            })
    return {"pairs": pairs}

def handle_translate(params):
    argos = get_argos()
    if not argos:
        raise RuntimeError("Argos Translate is not installed in the Python runtime.")
    from_code = params.get("from")
    to_code = params.get("to")
    text = params.get("text", "")
    
    installed_languages = argos.translate.get_installed_languages()
    from_lang = next((l for l in installed_languages if l.code == from_code), None)
    to_lang = next((l for l in installed_languages if l.code == to_code), None)
    
    if not from_lang or not to_lang:
        raise ValueError(f"Language pair {from_code} -> {to_code} is not installed.")
        
    translation = from_lang.get_translation(to_lang)
    if not translation:
        raise ValueError(f"Language pair {from_code} -> {to_code} is not installed.")
        
    translated_text = translation.translate(text)
    return {"translatedText": translated_text}

def handle_install_model(params):
    argos = get_argos()
    if not argos:
        raise RuntimeError("Argos Translate is not installed.")
    model_path = params.get("path")
    argos.package.install_from_path(model_path)
    return {"installed": True}

def handle_uninstall_model(params):
    argos = get_argos()
    if not argos:
        raise RuntimeError("Argos Translate is not installed.")
    from_code = params.get("from")
    to_code = params.get("to")
    installed_packages = argos.package.get_installed_packages()
    for pkg in installed_packages:
        if pkg.from_code == from_code and pkg.to_code == to_code:
            argos.package.uninstall(pkg)
            return {"uninstalled": True}
    return {"uninstalled": False}

DISPATCH = {
    "ping": handle_ping,
    "list_installed_pairs": handle_list_installed_pairs,
    "translate": handle_translate,
    "install_model": handle_install_model,
    "uninstall_model": handle_uninstall_model
}

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
            req_id = req.get("id")
            action = req.get("action")
            params = req.get("params", {})
            handler = DISPATCH.get(action)
            if not handler:
                raise ValueError(f"Unknown action: {action}")
            result = handler(params)
            resp = {"id": req_id, "version": 1, "success": True, "result": result}
        except Exception as e:
            sys.stderr.write(f"Worker error: {str(e)}\n")
            sys.stderr.flush()
            resp = {"id": req.get("id") if 'req' in locals() else None, "version": 1, "success": False, "error": {"message": str(e)}}
        sys.stdout.write(json.dumps(resp) + "\n")
        sys.stdout.flush()

if __name__ == '__main__':
    main()
```

Create `src/infrastructure/translation/ArgosProcessWorker.h`:

```cpp
#pragma once

#include "core/errors/AppError.h"
#include "core/types/Result.h"
#include "core/types/ServiceStatus.h"

#include <QJsonObject>
#include <QProcess>
#include <QString>
#include <QStringList>

#include <memory>
#include <mutex>

namespace Lexicon {

class ArgosProcessWorker {
  public:
    explicit ArgosProcessWorker(QString program = QStringLiteral("python3"),
                                QStringList arguments = {QStringLiteral(LEXICON_SOURCE_DIR "/scripts/argos_worker.py")});
    ~ArgosProcessWorker();

    ServiceStatus start();
    void stop();
    [[nodiscard]] bool isRunning() const;

    Result<QJsonObject> sendRequest(const QString& action, const QJsonObject& params, int timeoutMs = 5000);

  private:
    QString m_program;
    QStringList m_arguments;
    std::unique_ptr<QProcess> m_process;
    int m_requestId = 0;
    std::mutex m_mutex;
};

} // namespace Lexicon
```

Create `src/infrastructure/translation/ArgosProcessWorker.cpp`:

```cpp
#include "infrastructure/translation/ArgosProcessWorker.h"
#include "infrastructure/logging/Logging.h"

#include <QJsonDocument>
#include <QJsonObject>

namespace Lexicon {

ArgosProcessWorker::ArgosProcessWorker(QString program, QStringList arguments)
    : m_program(std::move(program)), m_arguments(std::move(arguments)) {}

ArgosProcessWorker::~ArgosProcessWorker() {
    stop();
}

ServiceStatus ArgosProcessWorker::start() {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (m_process && m_process->state() == QProcess::Running) {
        return ServiceStatus::success();
    }

    m_process = std::make_unique<QProcess>();
    m_process->setProgram(m_program);
    m_process->setArguments(m_arguments);

    QObject::connect(m_process.get(), &QProcess::readyReadStandardError, [this] {
        const QByteArray errData = m_process->readAllStandardError();
        qCDebug(lexiconApp) << "event=argos_worker_stderr output=" << errData.trimmed();
    });

    m_process->start();
    if (!m_process->waitForStarted(2000)) {
        return ServiceStatus::failure(AppError::make(
            ErrorCode::Unknown, QStringLiteral("Failed to start Argos Python worker process."), m_process->errorString()));
    }

    return ServiceStatus::success();
}

void ArgosProcessWorker::stop() {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (!m_process) {
        return;
    }
    if (m_process->state() == QProcess::Running) {
        m_process->terminate();
        if (!m_process->waitForFinished(1000)) {
            m_process->kill();
        }
    }
    m_process.reset();
}

bool ArgosProcessWorker::isRunning() const {
    return m_process && m_process->state() == QProcess::Running;
}

Result<QJsonObject> ArgosProcessWorker::sendRequest(const QString& action, const QJsonObject& params, int timeoutMs) {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (!isRunning()) {
        const auto status = start();
        if (!status.available) {
            return Result<QJsonObject>(status.error);
        }
    }

    const int id = ++m_requestId;
    QJsonObject req{
        {QStringLiteral("id"), id},
        {QStringLiteral("version"), 1},
        {QStringLiteral("action"), action},
        {QStringLiteral("params"), params},
    };

    const QByteArray requestData = QJsonDocument(req).toJson(QJsonDocument::Compact) + '\n';
    if (m_process->write(requestData) == -1) {
        return Result<QJsonObject>(AppError::make(
            ErrorCode::Unknown, QStringLiteral("Failed to write to Argos worker stdin.")));
    }

    if (!m_process->waitForReadyRead(timeoutMs)) {
        m_process->kill();
        return Result<QJsonObject>(AppError::make(
            ErrorCode::Unknown, QStringLiteral("Argos worker timed out responding.")));
    }

    const QByteArray line = m_process->readLine().trimmed();
    QJsonParseError parseError;
    const QJsonDocument doc = QJsonDocument::fromJson(line, &parseError);
    if (doc.isNull() || !doc.isObject()) {
        return Result<QJsonObject>(AppError::make(
            ErrorCode::ConfigParseError, QStringLiteral("Malformed JSON response from worker."), line));
    }

    const QJsonObject obj = doc.object();
    if (!obj.value(QStringLiteral("success")).toBool()) {
        const QString errorMsg = obj.value(QStringLiteral("error")).toObject().value(QStringLiteral("message")).toString();
        return Result<QJsonObject>(AppError::make(
            ErrorCode::DatabaseUnavailable, errorMsg));
    }

    return Result<QJsonObject>(obj.value(QStringLiteral("result")).toObject());
}

} // namespace Lexicon
```

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_argos_worker_protocol --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add scripts/argos_worker.py src/infrastructure/translation/ArgosProcessWorker.h src/infrastructure/translation/ArgosProcessWorker.cpp src/infrastructure/CMakeLists.txt tests/unit/test_argos_worker_protocol.cpp tests/CMakeLists.txt
git commit -m "feat(translation): implement local Argos Python worker and JSON protocol"
```

---

### Task 6: Argos Translation Service and Controller Integration

**Files:**
- Create: `src/core/interfaces/TranslationService.h`
- Create: `src/core/models/LanguagePair.h`
- Create: `src/infrastructure/translation/ArgosTranslationService.h`
- Create: `src/infrastructure/translation/ArgosTranslationService.cpp`
- Modify: `src/app/ApplicationController.h:20-55`
- Modify: `src/app/ApplicationController.cpp:120-176`
- Modify: `src/app/Application.cpp:68-81`
- Create: `tests/unit/test_translation_service.cpp`

**Interfaces:**
- `TranslationService` Interface:
  - `translate(const QString& text, const QString& from, const QString& to) -> Result<QString>`
  - `installedPairs() -> Result<QList<LanguagePair>>`
  - `isPairInstalled(const QString& from, const QString& to) -> bool`

- [ ] **Step 1: Write unit test for TranslationService**

Create `tests/unit/test_translation_service.cpp`:

```cpp
#include "infrastructure/translation/ArgosTranslationService.h"
#include <QTest>

using namespace Lexicon;

class TranslationServiceTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void detectsLanguagePairAvailability();
    void handlesEmptyInputGracefully();
};

void TranslationServiceTest::detectsLanguagePairAvailability() {
    ArgosTranslationService service;
    // Without pairs installed, should report false safely
    QVERIFY(!service.isPairInstalled(QStringLiteral("en"), QStringLiteral("fr")));
}

void TranslationServiceTest::handlesEmptyInputGracefully() {
    ArgosTranslationService service;
    const auto result = service.translate(QStringLiteral("   "), QStringLiteral("en"), QStringLiteral("es"));
    QVERIFY(!result.hasValue());
    QCOMPARE(result.error().code, ErrorCode::InvalidArgument);
}

QTEST_GUILESS_MAIN(TranslationServiceTest)
#include "test_translation_service.moc"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cmake --build --preset dev-debug --target test_translation_service`
Expected: FAIL (missing header/implementation).

- [ ] **Step 3: Implement TranslationService interface and ArgosTranslationService**

Create `src/core/models/LanguagePair.h`:
```cpp
#pragma once
#include <QString>

namespace Lexicon {
struct LanguagePair {
    QString source;
    QString target;
    QString sourceName;
    QString targetName;
};
}
```

Create `src/core/interfaces/TranslationService.h`:
```cpp
#pragma once
#include "core/models/LanguagePair.h"
#include "core/types/Result.h"
#include <QList>
#include <QString>

namespace Lexicon {
class TranslationService {
  public:
    virtual ~TranslationService() = default;
    virtual Result<QString> translate(const QString& text, const QString& from, const QString& to) = 0;
    virtual Result<QList<LanguagePair>> installedPairs() = 0;
    virtual bool isPairInstalled(const QString& from, const QString& to) = 0;
};
}
```

Create `src/infrastructure/translation/ArgosTranslationService.h` and `.cpp` wrapping `ArgosProcessWorker`.

In `ApplicationController.cpp`:
Integrate translation call when `sourceLanguage != targetLanguage`:
```cpp
if (m_config.dictionary.sourceLanguage != m_config.dictionary.targetLanguage) {
    if (m_translationService && m_translationService->isPairInstalled(m_config.dictionary.sourceLanguage, m_config.dictionary.targetLanguage)) {
        const auto transResult = m_translationService->translate(trimmed, m_config.dictionary.sourceLanguage, m_config.dictionary.targetLanguage);
        if (transResult.hasValue()) {
            result.entry->metadata.insert(QStringLiteral("translation"), transResult.value());
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_translation_service --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add src/core/interfaces/TranslationService.h src/core/models/LanguagePair.h src/infrastructure/translation/ArgosTranslationService.h src/infrastructure/translation/ArgosTranslationService.cpp src/app/ApplicationController.h src/app/ApplicationController.cpp src/app/Application.cpp tests/unit/test_translation_service.cpp
git commit -m "feat(translation): integrate ArgosTranslationService into ApplicationController"
```

---

### Task 7: Static Language Pack Catalog and Download Manager

**Files:**
- Create: `data/catalog/catalog.schema.json`
- Create: `src/infrastructure/translation/LanguagePackManager.h`
- Create: `src/infrastructure/translation/LanguagePackManager.cpp`
- Modify: `src/infrastructure/CMakeLists.txt`
- Create: `tests/unit/test_language_pack_manager.cpp`

**Interfaces:**
- Struct `LanguagePackInfo`:
  `QString id; QString source; QString target; QString versionName; QString url; qint64 size; QString sha256; QString description;`
- Class `LanguagePackManager`:
  - `fetchCatalog(const QUrl& catalogUrl) -> void`
  - `installPack(const LanguagePackInfo& pack) -> void`
  - `cancelDownload(const QString& packId) -> void`
  - `uninstallPack(const QString& source, const QString& target) -> void`
  - Signals: `catalogUpdated(QList<LanguagePackInfo>)`, `downloadProgress(QString packId, qint64 bytesReceived, qint64 bytesTotal)`, `installationCompleted(QString packId)`, `operationFailed(QString packId, QString error)`

- [ ] **Step 1: Write unit test for catalog parsing, checksum verification, and download cancellation**

Create `tests/unit/test_language_pack_manager.cpp`:

```cpp
#include "infrastructure/translation/LanguagePackManager.h"
#include <QCryptographicHash>
#include <QTemporaryFile>
#include <QTest>

using namespace Lexicon;

class LanguagePackManagerTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void parsesCatalogJsonCorrectly();
    void verifiesChecksumCorrectly();
    void detectsChecksumMismatch();
};

void LanguagePackManagerTest::parsesCatalogJsonCorrectly() {
    const QByteArray catalogData = R"({
        "version": 1,
        "packs": [
            {
                "id": "translate-en_es",
                "source": "en",
                "target": "es",
                "version": "1.9.0",
                "url": "https://example.com/en_es.argosmodel",
                "size": 85000000,
                "sha256": "4a5b6c"
            }
        ]
    })";

    const auto packs = LanguagePackManager::parseCatalogJson(catalogData);
    QCOMPARE(packs.size(), 1);
    QCOMPARE(packs.constFirst().id, QStringLiteral("translate-en_es"));
    QCOMPARE(packs.constFirst().source, QStringLiteral("en"));
    QCOMPARE(packs.constFirst().target, QStringLiteral("es"));
    QCOMPARE(packs.constFirst().sha256, QStringLiteral("4a5b6c"));
}

void LanguagePackManagerTest::verifiesChecksumCorrectly() {
    QTemporaryFile temp;
    QVERIFY(temp.open());
    temp.write("model payload");
    temp.flush();

    const QByteArray expectedSha256 = QCryptographicHash::hash("model payload", QCryptographicHash::Sha256).toHex();
    QVERIFY(LanguagePackManager::verifyFileIntegrity(temp.fileName(), QString::fromLatin1(expectedSha256)));
}

void LanguagePackManagerTest::detectsChecksumMismatch() {
    QTemporaryFile temp;
    QVERIFY(temp.open());
    temp.write("model payload");
    temp.flush();

    QVERIFY(!LanguagePackManager::verifyFileIntegrity(temp.fileName(), QStringLiteral("0000000000000000000000000000000000000000000000000000000000000000")));
}

QTEST_GUILESS_MAIN(LanguagePackManagerTest)
#include "test_language_pack_manager.moc"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cmake --build --preset dev-debug --target test_language_pack_manager`
Expected: FAIL (`LanguagePackManager` missing).

- [ ] **Step 3: Implement LanguagePackManager with Qt Network & Cryptographic verification**

Create `src/infrastructure/translation/LanguagePackManager.h`:
```cpp
#pragma once

#include "core/types/Result.h"

#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QObject>
#include <QString>

namespace Lexicon {

struct LanguagePackInfo {
    QString id;
    QString source;
    QString target;
    QString version;
    QString url;
    qint64 size = 0;
    QString sha256;
    QString description;
};

class LanguagePackManager : public QObject {
    Q_OBJECT

  public:
    explicit LanguagePackManager(QObject* parent = nullptr);

    static QList<LanguagePackInfo> parseCatalogJson(const QByteArray& jsonData);
    static bool verifyFileIntegrity(const QString& filePath, const QString& expectedSha256);

    void fetchCatalog(const QUrl& url);
    void installPack(const LanguagePackInfo& pack);
    void cancelDownload(const QString& packId);

  Q_SIGNALS:
    void catalogLoaded(const QList<LanguagePackInfo>& packs);
    void downloadProgress(const QString& packId, qint64 received, qint64 total);
    void installationFinished(const QString& packId);
    void operationError(const QString& packId, const QString& errorMessage);

  private:
    QNetworkAccessManager m_network;
    QHash<QString, QNetworkReply*> m_activeDownloads;
};

} // namespace Lexicon
```

Implement in `src/infrastructure/translation/LanguagePackManager.cpp` using `QNetworkAccessManager::get` and `QCryptographicHash`.

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_language_pack_manager --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add data/catalog/catalog.schema.json src/infrastructure/translation/LanguagePackManager.h src/infrastructure/translation/LanguagePackManager.cpp src/infrastructure/CMakeLists.txt tests/unit/test_language_pack_manager.cpp tests/CMakeLists.txt
git commit -m "feat(translation): implement static language pack catalog downloader and SHA-256 verifier"
```

---

### Task 8: UI Popup Enhancements for IPA & Translations

**Files:**
- Modify: `ui/popup/LookupPopup.qml:167-215`
- Modify: `ui/components/DefinitionList.qml:20-44`
- Test: `tests/integration/test_qml_views.cpp`

**Interfaces:**
- Consumes:
  - `entry.pronunciation.text` / `entry.pronunciation.ipa`
  - `entry.metadata.translation`
- Produces:
  - Clean pronunciation display label (e.g. `[ˈsɛrənˌdɪpɪti]`)
  - Optional translation label when `entry.metadata.translation` exists
  - Definitions grouped by POS

- [ ] **Step 1: Write failing QML integration test for IPA and translation rendering**

In `tests/integration/test_qml_views.cpp`:

```cpp
void QmlViewsTest::lookupPopupRendersIpaAndTranslation() {
    QQuickView view;
    configureView(view);
    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);

    QVariantMap pronunciation{
        {QStringLiteral("ipa"), QStringLiteral("/ˈsɛrənˌdɪpɪti/")},
        {QStringLiteral("text"), QStringLiteral("/ˈsɛrənˌdɪpɪti/")},
        {QStringLiteral("style"), QStringLiteral("ipa")}
    };
    QVariantMap metadata{
        {QStringLiteral("translation"), QStringLiteral("serendipidad")}
    };
    QVariantMap entry{
        {QStringLiteral("word"), QStringLiteral("serendipity")},
        {QStringLiteral("pronunciation"), pronunciation},
        {QStringLiteral("metadata"), metadata},
        {QStringLiteral("definitions"), QVariantList{}}
    };

    view.rootObject()->setProperty("loading", false);
    view.rootObject()->setProperty("hasMessage", false);
    view.rootObject()->setProperty("word", QStringLiteral("serendipity"));
    view.rootObject()->setProperty("entry", entry);
    QCoreApplication::processEvents();

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.rootObject()->property("entry").isValid());
}
```

- [ ] **Step 2: Run test to verify it passes or fails**

Run: `ctest --test-dir build/dev-debug -R test_qml_views --output-on-failure`

- [ ] **Step 3: Update `LookupPopup.qml` and `DefinitionList.qml`**

In `ui/popup/LookupPopup.qml`:
Update pronunciation section to show formatted IPA:
```qml
Label {
    visible: root.entry !== null
             && root.entry !== undefined
             && root.entry.pronunciation !== undefined
             && root.entry.pronunciation.text !== ""
    text: root.entry && root.entry.pronunciation ? root.entry.pronunciation.text : ""
    color: theme.textSecondary
    font.pixelSize: 13
    font.family: "Noto Sans, DejaVu Sans, sans-serif"
    wrapMode: Text.Wrap
    Layout.fillWidth: true
}

// Translation section if present
Rectangle {
    visible: root.entry && root.entry.metadata && root.entry.metadata.translation
    Layout.fillWidth: true
    radius: 4
    color: Qt.rgba(theme.accent.r, theme.accent.g, theme.accent.b, 0.1)
    implicitHeight: transCol.implicitHeight + 12
    ColumnLayout {
        id: transCol
        anchors.fill: parent
        anchors.margins: 6
        Label {
            text: qsTr("Translation")
            font.pixelSize: 10
            font.bold: true
            color: theme.accent
        }
        Label {
            text: root.entry && root.entry.metadata ? root.entry.metadata.translation : ""
            color: theme.textPrimary
            font.pixelSize: 13
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_qml_views --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add ui/popup/LookupPopup.qml ui/components/DefinitionList.qml tests/integration/test_qml_views.cpp
git commit -m "feat(ui): display IPA pronunciation and translation card in lookup popup"
```

---

### Task 9: Settings View Language Pack Management UI

**Files:**
- Modify: `ui/settings/SettingsView.qml:136-158`
- Modify: `src/app/Application.cpp:50-80`
- Test: `tests/integration/test_qml_views.cpp`

**Interfaces:**
- Exposes `languageManager` context object to QML with methods:
  - `install(QString packId)`
  - `remove(QString source, QString target)`
  - `refreshCatalog()`
  - Properties: `installedPacks`, `availablePacks`, `downloadProgress`

- [ ] **Step 1: Write integration test for SettingsView language section**

In `tests/integration/test_qml_views.cpp`:

```cpp
void QmlViewsTest::settingsViewShowsLanguageSection() {
    QQuickView view;
    configureView(view);
    view.setSource(sourceUrl(QStringLiteral("ui/settings/SettingsView.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.rootObject() != nullptr);
}
```

- [ ] **Step 2: Run test to verify it runs**

Run: `ctest --test-dir build/dev-debug -R test_qml_views --output-on-failure`

- [ ] **Step 3: Update `SettingsView.qml` to include Language Packs card**

Add SectionCard for "Language Packs & Translation" displaying:
- Installed language models with remove buttons
- Available downloadable models with progress bars and install buttons
- Refresh catalog button

- [ ] **Step 4: Run test to verify it passes**

Run: `ctest --test-dir build/dev-debug -R test_qml_views --output-on-failure`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add ui/settings/SettingsView.qml src/app/Application.cpp tests/integration/test_qml_views.cpp
git commit -m "feat(ui): add language pack management section to settings view"
```

---

### Task 10: Documentation & Quality Verification

**Files:**
- Create: `docs/wordnet-dictionary.md`
- Create: `docs/pronunciation-ipa.md`
- Create: `docs/argos-translation.md`
- Create: `docs/language-packs.md`
- Modify: `docs/configuration.md`
- Modify: `docs/dictionary-format.md`
- Modify: `ROADMAP.md`
- Modify: `README.md`

- [ ] **Step 1: Document WordNet 3.0 provenance, license, schema, and import reproducibility**

Create `docs/wordnet-dictionary.md` detailing:
- Dataset: Princeton WordNet 3.0
- License: WordNet 3.0 License (BSD-compatible)
- Attribution: "WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved."
- Reproduction command: `python3 -m scripts.lexicon_importer.import_wordnet --wordnet-dir ... --cmudict-file ... --output data/dictionaries/en.sqlite`

- [ ] **Step 2: Document CMUdict IPA conversion methodology, stress rules, and limitations**

Create `docs/pronunciation-ipa.md` detailing:
- Dataset: CMU Pronouncing Dictionary 0.7b
- License: BSD 2-Clause
- ARPABET to IPA phoneme mappings table
- Primary (`ˈ`) and secondary (`ˌ`) stress rules
- Limitations: North American English emphasis, lack of inflected forms/proper nouns, future fallback strategy (Wiktionary, Epitran G2P)

- [ ] **Step 3: Document Argos Translation architecture, worker JSON protocol, and packaging**

Create `docs/argos-translation.md` and `docs/language-packs.md` detailing:
- Local `QProcess` Python worker architecture
- JSON stdin/stdout protocol specification
- Offline-only guarantees (no background downloads or sockets)
- Static catalog JSON format and hosting
- Packaging instructions and SHA-256 verification

- [ ] **Step 4: Update README.md, ROADMAP.md, and configuration reference**

Reflect offline English dictionary, IPA pronunciation, and Argos translation pack support.

- [ ] **Step 5: Run CI quality gates**

Run:
```bash
just format-check
just qml-lint
just test
```
Expected: All gates pass with 0 errors and 0 warnings.

- [ ] **Step 6: Commit documentation**

```bash
git add docs/ README.md ROADMAP.md
git commit -m "docs: document WordNet, CMUdict IPA, and Argos translation features"
```

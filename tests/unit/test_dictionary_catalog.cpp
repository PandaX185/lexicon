#include "dictionary/CatalogDictionaryRepository.h"
#include "dictionary/DictionaryCatalog.h"

#include <QFile>
#include <QFileInfo>
#include <QHash>
#include <QTemporaryDir>
#include <QTest>

#include <memory>
#include <utility>

using namespace Lexicon;

namespace {

using DictionaryInfos = QHash<QString, DictionaryInfo>;

DictionaryInfo info(const char* name, const char* source, const char* target) {
    return DictionaryInfo{
        .name = QString::fromUtf8(name),
        .sourceLanguage = QString::fromUtf8(source),
        .targetLanguage = QString::fromUtf8(target),
        .license = {},
        .attribution = {},
    };
}

// Describes itself from a path -> info map, so catalog behaviour can be tested
// without SQLite: a file without an entry is "not a Lexicon dictionary".
class StubRepository final : public DictionaryRepository {
  public:
    StubRepository(QString path, DictionaryInfos infos) : m_path(std::move(path)), m_infos(std::move(infos)) {}

    [[nodiscard]] DictionaryLookupResult findByWord(const QString& word) const override {
        ++calls;
        WordEntry entry;
        entry.word = QFileInfo(m_path).completeBaseName() + QLatin1Char(':') + word;
        Definition definition;
        definition.text = QStringLiteral("Definition from the resolved dictionary.");
        entry.definitions.append(definition);
        return DictionaryLookupResult::success(std::move(entry));
    }

    [[nodiscard]] DictionaryDescription describe() const override {
        const auto found = m_infos.constFind(QFileInfo(m_path).completeBaseName());
        if (found == m_infos.cend()) {
            return DictionaryDescription::failure(
                AppError::make(ErrorCode::DatabaseQueryFailed, QStringLiteral("Not a Lexicon dictionary.")));
        }
        DictionaryDescription description;
        description.info = found.value();
        description.entryCount = 7;
        return description;
    }

    [[nodiscard]] QString location() const override {
        return m_path;
    }

    mutable int calls = 0;

  private:
    QString m_path;
    DictionaryInfos m_infos;
};

DictionaryCatalog::RepositoryFactory stubFactory(DictionaryInfos infos) {
    return [infos = std::move(infos)](const QString& path) -> std::unique_ptr<DictionaryRepository> {
        return std::make_unique<StubRepository>(path, infos);
    };
}

void writeFile(const QTemporaryDir& directory, const QString& name) {
    QFile file(directory.filePath(name));
    QVERIFY(file.open(QIODevice::WriteOnly));
    file.write("not a real database");
}

} // namespace

class DictionaryCatalogTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void listsOnlyDescribableDictionaries();
    void sortsEntriesByFileName();
    void resolvesExactPair();
    void normalizesRequestedLanguageTags();
    void fallsBackToSourceMonolingualDictionary();
    void fallsBackToTargetMonolingualDictionary();
    void reportsUninstalledPair();
    void delegatesLookupToResolvedDictionary();
    void describesResolvedDictionary();
    void locationFallsBackToDirectory();
    void emptyDirectoryHasNoEntries();
    void scansMultipleDirectoriesWithPriority();
};

void DictionaryCatalogTest::listsOnlyDescribableDictionaries() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));
    writeFile(directory, QStringLiteral("stray.sqlite"));
    writeFile(directory, QStringLiteral("notes.txt"));

    const DictionaryCatalog catalog(directory.path(),
                                    stubFactory({{QStringLiteral("english"), info("English", "en", "en")}}));

    QCOMPARE(catalog.entries().size(), 1);
    QCOMPARE(catalog.entries().constFirst().info.name, QStringLiteral("English"));
    QVERIFY(catalog.entries().constFirst().path.endsWith(QStringLiteral("english.sqlite")));
    QCOMPARE(catalog.entries().constFirst().entryCount, 7);
}

void DictionaryCatalogTest::sortsEntriesByFileName() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("b-german.sqlite"));
    writeFile(directory, QStringLiteral("a-english.sqlite"));

    const DictionaryCatalog catalog(directory.path(), stubFactory({
                                                          {QStringLiteral("a-english"), info("English", "en", "en")},
                                                          {QStringLiteral("b-german"), info("German", "de", "de")},
                                                      }));

    QCOMPARE(catalog.entries().size(), 2);
    QCOMPARE(catalog.entries().at(0).info.name, QStringLiteral("English"));
    QCOMPARE(catalog.entries().at(1).info.name, QStringLiteral("German"));
}

void DictionaryCatalogTest::resolvesExactPair() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));
    writeFile(directory, QStringLiteral("english-german.sqlite"));

    const DictionaryCatalog catalog(directory.path(),
                                    stubFactory({
                                        {QStringLiteral("english"), info("English", "en", "en")},
                                        {QStringLiteral("english-german"), info("English to German", "en", "de")},
                                    }));

    const auto entry = catalog.find(QStringLiteral("en"), QStringLiteral("de"));
    QVERIFY(entry.has_value());
    QCOMPARE(entry->info.name, QStringLiteral("English to German"));
}

void DictionaryCatalogTest::normalizesRequestedLanguageTags() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english-german.sqlite"));

    const DictionaryCatalog catalog(
        directory.path(), stubFactory({{QStringLiteral("english-german"), info("English to German", "en", "de")}}));

    const auto entry = catalog.find(QStringLiteral(" EN "), QStringLiteral("DE"));
    QVERIFY(entry.has_value());
    QCOMPARE(entry->info.name, QStringLiteral("English to German"));
}

void DictionaryCatalogTest::fallsBackToSourceMonolingualDictionary() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));

    const DictionaryCatalog catalog(directory.path(),
                                    stubFactory({{QStringLiteral("english"), info("English", "en", "en")}}));

    const auto entry = catalog.find(QStringLiteral("en"), QStringLiteral("fr"));
    QVERIFY(entry.has_value());
    QCOMPARE(entry->info.name, QStringLiteral("English"));
}

void DictionaryCatalogTest::fallsBackToTargetMonolingualDictionary() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("german.sqlite"));

    const DictionaryCatalog catalog(directory.path(),
                                    stubFactory({{QStringLiteral("german"), info("German", "de", "de")}}));

    const auto entry = catalog.find(QStringLiteral("fr"), QStringLiteral("de"));
    QVERIFY(entry.has_value());
    QCOMPARE(entry->info.name, QStringLiteral("German"));
}

void DictionaryCatalogTest::reportsUninstalledPair() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));

    const DictionaryCatalog catalog(directory.path(),
                                    stubFactory({{QStringLiteral("english"), info("English", "en", "en")}}));

    QVERIFY(!catalog.find(QStringLiteral("ja"), QStringLiteral("ko")).has_value());
}

void DictionaryCatalogTest::delegatesLookupToResolvedDictionary() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));
    writeFile(directory, QStringLiteral("english-german.sqlite"));

    const DictionaryInfos infos{
        {QStringLiteral("english"), info("English", "en", "en")},
        {QStringLiteral("english-german"), info("English to German", "en", "de")},
    };
    const DictionaryCatalog catalog(directory.path(), stubFactory(infos));

    CatalogDictionaryRepository repository(catalog, stubFactory(infos), QStringLiteral("en"), QStringLiteral("de"));
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("serendipity"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("english-german:serendipity"));
    QVERIFY(repository.location().endsWith(QStringLiteral("english-german.sqlite")));
}

void DictionaryCatalogTest::describesResolvedDictionary() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    writeFile(directory, QStringLiteral("english.sqlite"));

    const DictionaryInfos infos{{QStringLiteral("english"), info("English", "en", "en")}};
    const DictionaryCatalog catalog(directory.path(), stubFactory(infos));

    CatalogDictionaryRepository repository(catalog, stubFactory(infos), QStringLiteral("en"), QStringLiteral("en"));
    const DictionaryDescription description = repository.describe();

    QVERIFY(description.ok());
    QCOMPARE(description.info.name, QStringLiteral("English"));
    QCOMPARE(description.entryCount, 7);
}

void DictionaryCatalogTest::locationFallsBackToDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const DictionaryInfos infos;
    const DictionaryCatalog catalog(directory.path(), stubFactory(infos));
    CatalogDictionaryRepository repository(catalog, stubFactory(infos), QStringLiteral("en"), QStringLiteral("de"));

    QCOMPARE(repository.location(), catalog.directory());

    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("serendipity"));
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
    QVERIFY(result.error.message.contains(QStringLiteral("en")));
}

void DictionaryCatalogTest::emptyDirectoryHasNoEntries() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const DictionaryCatalog catalog(directory.path(), stubFactory({}));

    QVERIFY(catalog.entries().isEmpty());
    QVERIFY(!catalog.find(QStringLiteral("en"), QStringLiteral("en")).has_value());
}

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

    const QString userDirPath = userDir.path();
    auto factory = [systemInfos, userInfos, userDirPath](const QString& path) -> std::unique_ptr<DictionaryRepository> {
        const QString base = QFileInfo(path).completeBaseName();
        if (path.contains(QStringLiteral("user")) || path.startsWith(userDirPath)) {
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

QTEST_GUILESS_MAIN(DictionaryCatalogTest)

#include "test_dictionary_catalog.moc"

#include "infrastructure/database/SqliteDictionaryRepository.h"
#include "support/SqliteFixture.h"

#include <QFile>
#include <QFuture>
#include <QSqlDatabase>
#include <QSqlError>
#include <QSqlQuery>
#include <QTemporaryDir>
#include <QTest>
#include <QtConcurrent/QtConcurrentRun>

#include <array>
#include <memory>

using namespace Lexicon;
using Lexicon::Test::SqliteFixture;

namespace {

void requireSqliteDriver() {
    if (!QSqlDatabase::isDriverAvailable(QStringLiteral("QSQLITE"))) {
        QSKIP("The Qt SQLite driver is not available.");
    }
}

void insertEntry(SqliteFixture& fixture, int id, const QString& word,
                 const QStringList& definitions = {QStringLiteral("A definition.")}, const QString& respelling = {},
                 const QString& ipa = {}, const QString& source = QStringLiteral("fixture"),
                 const QString& license = QStringLiteral("CC0-1.0")) {
    QSqlQuery entryQuery(fixture.database());
    entryQuery.prepare(QStringLiteral("INSERT INTO entries "
                                      "(id, word, pronunciation_respelling, pronunciation_ipa, source, license) "
                                      "VALUES (?, ?, ?, ?, ?, ?)"));
    entryQuery.addBindValue(id);
    entryQuery.addBindValue(word);
    // The schema marks pronunciation columns NOT NULL: a null QString would bind as SQL NULL.
    entryQuery.addBindValue(respelling.isNull() ? QStringLiteral("") : respelling);
    entryQuery.addBindValue(ipa.isNull() ? QStringLiteral("") : ipa);
    entryQuery.addBindValue(source);
    entryQuery.addBindValue(license);
    QVERIFY2(entryQuery.exec(), qPrintable(entryQuery.lastError().text()));

    int definitionId = id * 100;
    int sense = 0;
    for (const QString& definition : definitions) {
        QSqlQuery definitionQuery(fixture.database());
        definitionQuery.prepare(QStringLiteral("INSERT INTO definitions "
                                               "(id, entry_id, part_of_speech, definition, label, sense_order) "
                                               "VALUES (?, ?, 'noun', ?, '', ?)"));
        definitionQuery.addBindValue(definitionId);
        definitionQuery.addBindValue(id);
        definitionQuery.addBindValue(definition);
        definitionQuery.addBindValue(sense);
        QVERIFY2(definitionQuery.exec(), qPrintable(definitionQuery.lastError().text()));
        ++definitionId;
        ++sense;
    }
}

} // namespace

class SqliteRepositoryTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void looksUpEntryCaseInsensitively();
    void returnsDefinitionsInSenseOrder();
    void returnsEntryMetadata();
    void returnsEntryWithoutDefinitions();
    void reportsMissingDatabase();
    void reportsEmptyDatabasePath();
    void reportsDirectoryPath();
    void reportsEmptyDatabaseFile();
    void reportsCorruptDatabase();
    void reportsUnknownWord();
    void handlesUnicodeWord();
    void handlesSqlMetacharacters();
    void handlesConcurrentLookups();
    void removesLookupConnections();
    void exposesDatabaseLocation();
};

void SqliteRepositoryTest::looksUpEntryCaseInsensitively() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("serendipity"), {QStringLiteral("A fortunate discovery by chance.")},
                QStringLiteral("ser-uhn-DIP-i-tee"));

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("SERENDIPITY"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("serendipity"));
    QCOMPARE(result.entry->pronunciation.respelling, QStringLiteral("ser-uhn-DIP-i-tee"));
    QCOMPARE(result.entry->definitions.size(), 1);
    QCOMPARE(result.entry->definitions.constFirst().partOfSpeech, PartOfSpeech::Noun);
}

void SqliteRepositoryTest::returnsDefinitionsInSenseOrder() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("run"), {});

    QVERIFY(fixture.exec(
        QStringLiteral("INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order) VALUES "
                       "(12, 1, 'noun', 'Third sense.', '', 2),"
                       "(10, 1, 'verb', 'First sense.', '', 0),"
                       "(11, 1, 'noun', 'Second sense.', 'figurative', 1)")));

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("run"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->definitions.size(), 3);
    QCOMPARE(result.entry->definitions.at(0).text, QStringLiteral("First sense."));
    QCOMPARE(result.entry->definitions.at(0).partOfSpeech, PartOfSpeech::Verb);
    QCOMPARE(result.entry->definitions.at(1).text, QStringLiteral("Second sense."));
    QCOMPARE(result.entry->definitions.at(1).label, QStringLiteral("figurative"));
    QCOMPARE(result.entry->definitions.at(2).text, QStringLiteral("Third sense."));
}

void SqliteRepositoryTest::returnsEntryMetadata() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("apple"), {QStringLiteral("A fruit.")}, {}, {},
                QStringLiteral("fixture-source"), QStringLiteral("CC0-1.0"));
    QVERIFY(fixture.exec(
        QStringLiteral("INSERT INTO entry_metadata (entry_id, key, value) VALUES (1, 'frequency', 'high')")));

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("apple"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->metadata.value(QStringLiteral("source")), QStringLiteral("fixture-source"));
    QCOMPARE(result.entry->metadata.value(QStringLiteral("license")), QStringLiteral("CC0-1.0"));
    QCOMPARE(result.entry->metadata.value(QStringLiteral("frequency")), QStringLiteral("high"));
}

void SqliteRepositoryTest::returnsEntryWithoutDefinitions() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("empty"), {});

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("empty"));

    QVERIFY(result.found());
    QVERIFY(result.entry->definitions.isEmpty());
}

void SqliteRepositoryTest::reportsMissingDatabase() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteDictionaryRepository repository(directory.filePath(QStringLiteral("missing.sqlite")));
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
}

void SqliteRepositoryTest::reportsEmptyDatabasePath() {
    SqliteDictionaryRepository repository{QString()};

    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
}

void SqliteRepositoryTest::reportsDirectoryPath() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteDictionaryRepository repository(directory.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
}

void SqliteRepositoryTest::reportsEmptyDatabaseFile() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("empty.sqlite"));

    QFile file(path);
    QVERIFY(file.open(QIODevice::WriteOnly));
    file.close();

    SqliteDictionaryRepository repository(path);
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QVERIFY(result.error.code == ErrorCode::DatabaseOpenFailed || result.error.code == ErrorCode::DatabaseQueryFailed);
}

void SqliteRepositoryTest::reportsCorruptDatabase() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("corrupt.sqlite"));

    QFile file(path);
    QVERIFY(file.open(QIODevice::WriteOnly));
    file.write("this is not a sqlite database");
    file.close();

    SqliteDictionaryRepository repository(path);
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QVERIFY(result.error.code == ErrorCode::DatabaseOpenFailed || result.error.code == ErrorCode::DatabaseQueryFailed);
}

void SqliteRepositoryTest::reportsUnknownWord() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("known"));

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("unknown"));

    QVERIFY(!result.found());
    QVERIFY(!result.error.isError());
}

void SqliteRepositoryTest::handlesUnicodeWord() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("café"));

    SqliteDictionaryRepository repository(fixture.path());
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("café"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("café"));
}

void SqliteRepositoryTest::handlesSqlMetacharacters() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("o'clock"), {QStringLiteral("A contraction.")});

    SqliteDictionaryRepository repository(fixture.path());

    const DictionaryLookupResult apostrophe = repository.findByWord(QStringLiteral("O'CLOCK"));
    QVERIFY(apostrophe.found());

    const DictionaryLookupResult injection = repository.findByWord(QStringLiteral("x'); DROP TABLE entries; --"));
    QVERIFY(!injection.found());
    QVERIFY(!injection.error.isError());

    const DictionaryLookupResult stillThere = repository.findByWord(QStringLiteral("o'clock"));
    QVERIFY(stillThere.found());
}

void SqliteRepositoryTest::handlesConcurrentLookups() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("serendipity"), {QStringLiteral("A definition.")});

    SqliteDictionaryRepository repository(fixture.path());

    std::array<QFuture<DictionaryLookupResult>, 8> futures;
    for (QFuture<DictionaryLookupResult>& future : futures) {
        future = QtConcurrent::run([&repository] { return repository.findByWord(QStringLiteral("serendipity")); });
    }

    for (QFuture<DictionaryLookupResult>& future : futures) {
        future.waitForFinished();
        const DictionaryLookupResult result = future.result();
        QVERIFY(result.found());
        QCOMPARE(result.entry->word, QStringLiteral("serendipity"));
    }
}

void SqliteRepositoryTest::removesLookupConnections() {
    requireSqliteDriver();
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteFixture fixture(directory.filePath(QStringLiteral("dictionary.sqlite")));
    QVERIFY2(fixture.isOpen(), qPrintable(fixture.lastError()));
    QVERIFY(fixture.createSchema());
    insertEntry(fixture, 1, QStringLiteral("hello"));

    SqliteDictionaryRepository repository(fixture.path());
    QVERIFY(repository.findByWord(QStringLiteral("hello")).found());

    for (const QString& connectionName : QSqlDatabase::connectionNames()) {
        QVERIFY2(!connectionName.startsWith(QStringLiteral("lexicon_lookup_")),
                 qPrintable(QStringLiteral("Leaked connection: %1").arg(connectionName)));
    }
}

void SqliteRepositoryTest::exposesDatabaseLocation() {
    SqliteDictionaryRepository repository(QStringLiteral("/tmp/example.sqlite"));

    QCOMPARE(repository.location(), QStringLiteral("/tmp/example.sqlite"));
}

QTEST_GUILESS_MAIN(SqliteRepositoryTest)

#include "test_sqlite_repository.moc"

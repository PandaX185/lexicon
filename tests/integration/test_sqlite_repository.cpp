#include "infrastructure/database/SqliteDictionaryRepository.h"

#include <QFile>
#include <QSqlDatabase>
#include <QSqlError>
#include <QSqlQuery>
#include <QTemporaryDir>
#include <QTest>

using namespace Lexicon;

class SqliteRepositoryTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void looksUpEntryCaseInsensitively();
    void reportsMissingDatabase();
};

void SqliteRepositoryTest::looksUpEntryCaseInsensitively() {
    if (!QSqlDatabase::isDriverAvailable(QStringLiteral("QSQLITE"))) {
        QSKIP("The Qt SQLite driver is not available.");
    }

    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString databasePath = directory.filePath(QStringLiteral("dictionary.sqlite"));

    const QString connectionName = QStringLiteral("test_setup");
    {
        QSqlDatabase database = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), connectionName);
        database.setDatabaseName(databasePath);
        QVERIFY2(database.open(), qPrintable(database.lastError().text()));

        QFile schema(QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/schema.sql"));
        QVERIFY2(schema.open(QIODevice::ReadOnly), qPrintable(schema.errorString()));
        const QString schemaSql = QString::fromUtf8(schema.readAll());

        const QStringList statements = schemaSql.split(QLatin1Char(';'), Qt::SkipEmptyParts);
        for (const QString& statement : statements) {
            if (statement.trimmed().isEmpty()) {
                continue;
            }
            QSqlQuery query(database);
            QVERIFY2(query.exec(statement), qPrintable(query.lastError().text()));
        }

        QSqlQuery insertEntry(database);
        QVERIFY(insertEntry.exec(QStringLiteral(
            "INSERT INTO entries (id, word, pronunciation_respelling, pronunciation_ipa, source, license) "
            "VALUES (1, 'serendipity', 'ser-uhn-DIP-i-tee', '', 'fixture', 'CC0-1.0')")));

        QSqlQuery insertDefinition(database);
        QVERIFY(insertDefinition.exec(
            QStringLiteral("INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order) "
                           "VALUES (1, 1, 'noun', 'A fortunate discovery by chance.', '', 0)")));

        QSqlQuery insertMetadata(database);
        QVERIFY(insertMetadata.exec(
            QStringLiteral("INSERT INTO entry_metadata (entry_id, key, value) VALUES (1, 'fixture', 'true')")));

        database.close();
    }
    QSqlDatabase::removeDatabase(connectionName);

    SqliteDictionaryRepository repository(databasePath);
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("SERENDIPITY"));

    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("serendipity"));
    QCOMPARE(result.entry->pronunciation.respelling, QStringLiteral("ser-uhn-DIP-i-tee"));
    QCOMPARE(result.entry->definitions.size(), 1);
    QCOMPARE(result.entry->definitions.constFirst().partOfSpeech, PartOfSpeech::Noun);
    QCOMPARE(result.entry->metadata.value(QStringLiteral("fixture")), QStringLiteral("true"));
    QCOMPARE(repository.location(), databasePath);
}

void SqliteRepositoryTest::reportsMissingDatabase() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    SqliteDictionaryRepository repository(directory.filePath(QStringLiteral("missing.sqlite")));
    const DictionaryLookupResult result = repository.findByWord(QStringLiteral("hello"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
}

QTEST_GUILESS_MAIN(SqliteRepositoryTest)

#include "test_sqlite_repository.moc"

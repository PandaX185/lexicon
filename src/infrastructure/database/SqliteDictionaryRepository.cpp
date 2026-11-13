#include "infrastructure/database/SqliteDictionaryRepository.h"

#include "infrastructure/logging/Logging.h"

#include <QFileInfo>
#include <QSqlDatabase>
#include <QSqlError>
#include <QSqlQuery>
#include <QUuid>
#include <QVariant>

#include <utility>

namespace Lexicon {

namespace {

// The GUI thread never shares a QSqlDatabase with a worker thread: every lookup
// and description opens its own short-lived read-only connection and drops it
// again before returning.
class ReadOnlyConnection {
  public:
    ReadOnlyConnection(const QString& databasePath, const QString& connectionName)
        : m_connectionName(connectionName),
          m_database(QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), connectionName)) {
        m_database.setDatabaseName(databasePath);
        m_database.setConnectOptions(QStringLiteral("QSQLITE_OPEN_READONLY"));
    }

    ~ReadOnlyConnection() {
        if (m_database.isOpen()) {
            m_database.close();
        }
        m_database = QSqlDatabase();
        QSqlDatabase::removeDatabase(m_connectionName);
    }

    ReadOnlyConnection(const ReadOnlyConnection&) = delete;
    ReadOnlyConnection& operator=(const ReadOnlyConnection&) = delete;

    [[nodiscard]] bool open() {
        return m_database.open();
    }
    [[nodiscard]] QString lastError() const {
        return m_database.lastError().text();
    }
    [[nodiscard]] QSqlDatabase& database() {
        return m_database;
    }

  private:
    QString m_connectionName;
    QSqlDatabase m_database;
};

QString newConnectionName() {
    return QStringLiteral("lexicon_lookup_%1").arg(QUuid::createUuid().toString(QUuid::WithoutBraces));
}

// Shared by the headword, transliteration, and inflected-form queries so all of
// them read the same columns in the same order.
DictionaryLookupResult readEntry(QSqlDatabase& database, QSqlQuery& entryQuery, const QString& word,
                                 const QString& matchedForm = {}) {
    if (!entryQuery.exec()) {
        const QString detail = entryQuery.lastError().text();
        qCWarning(lexiconDatabase) << "event=entry_query_failed word_length=" << word.size() << "detail=" << detail;
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseQueryFailed, QStringLiteral("Could not query the dictionary database."), detail));
    }

    if (!entryQuery.next()) {
        return DictionaryLookupResult::notFound();
    }

    WordEntry entry;
    const qint64 entryId = entryQuery.value(0).toLongLong();
    entry.word = entryQuery.value(1).toString();
    entry.transliteration = entryQuery.value(2).toString();
    entry.matchedForm = matchedForm;
    entry.pronunciation.respelling = entryQuery.value(3).toString();
    entry.pronunciation.ipa = entryQuery.value(4).toString();

    const QString source = entryQuery.value(5).toString();
    const QString license = entryQuery.value(6).toString();
    if (!source.isEmpty()) {
        entry.metadata.insert(QStringLiteral("source"), source);
    }
    if (!license.isEmpty()) {
        entry.metadata.insert(QStringLiteral("license"), license);
    }

    QSqlQuery definitionQuery(database);
    definitionQuery.prepare(QStringLiteral("SELECT part_of_speech, definition, label FROM definitions "
                                           "WHERE entry_id = ? ORDER BY sense_order, id"));
    definitionQuery.addBindValue(entryId);

    if (!definitionQuery.exec()) {
        const QString detail = definitionQuery.lastError().text();
        qCWarning(lexiconDatabase) << "event=definition_query_failed detail=" << detail;
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseQueryFailed, QStringLiteral("Could not query dictionary definitions."), detail));
    }

    while (definitionQuery.next()) {
        Definition definition;
        definition.partOfSpeech = partOfSpeechFromString(definitionQuery.value(0).toString());
        definition.text = definitionQuery.value(1).toString();
        definition.label = definitionQuery.value(2).toString();
        entry.definitions.append(std::move(definition));
    }

    QSqlQuery metadataQuery(database);
    metadataQuery.prepare(QStringLiteral("SELECT key, value FROM entry_metadata WHERE entry_id = ?"));
    metadataQuery.addBindValue(entryId);

    if (metadataQuery.exec()) {
        while (metadataQuery.next()) {
            entry.metadata.insert(metadataQuery.value(0).toString(), metadataQuery.value(1).toString());
        }
    } else {
        qCWarning(lexiconDatabase) << "event=metadata_query_failed detail=" << metadataQuery.lastError().text();
    }

    return DictionaryLookupResult::success(std::move(entry));
}

} // namespace

SqliteDictionaryRepository::SqliteDictionaryRepository(QString databasePath)
    : m_databasePath(std::move(databasePath)) {}

DictionaryLookupResult SqliteDictionaryRepository::findByWord(const QString& word) const {
    if (m_databasePath.trimmed().isEmpty()) {
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary database path is configured.")));
    }

    const QFileInfo databaseFile(m_databasePath);
    if (!databaseFile.exists() || !databaseFile.isFile()) {
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("Dictionary database not found."), m_databasePath));
    }

    ReadOnlyConnection connection(m_databasePath, newConnectionName());
    if (!connection.open()) {
        const QString detail = connection.lastError();
        qCWarning(lexiconDatabase) << "event=database_open_failed path=" << m_databasePath << "detail=" << detail;
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open the dictionary database."), detail));
    }

    QSqlQuery entryQuery(connection.database());
    entryQuery.prepare(QStringLiteral("SELECT id, word, transliteration, pronunciation_respelling, pronunciation_ipa, "
                                      "source, license FROM entries WHERE word = ? COLLATE NOCASE LIMIT 1"));
    entryQuery.addBindValue(word);

    DictionaryLookupResult result = readEntry(connection.database(), entryQuery, word);
    if (result.error.isError() || result.found()) {
        return result;
    }

    // Romanized input: a dictionary for a non-Latin script indexes the Latin
    // transliteration alongside the headword. A dictionary that predates the
    // column simply has no transliteration to match, which is not an error.
    QSqlQuery transliterationQuery(connection.database());
    transliterationQuery.prepare(QStringLiteral("SELECT id, word, transliteration, pronunciation_respelling, "
                                                "pronunciation_ipa, source, license FROM entries "
                                                "WHERE transliteration = ? COLLATE NOCASE LIMIT 1"));
    transliterationQuery.addBindValue(word);

    if (!transliterationQuery.exec()) {
        qCDebug(lexiconDatabase) << "event=transliteration_query_unavailable detail="
                                 << transliterationQuery.lastError().text();
        transliterationQuery.finish();
        return DictionaryLookupResult::notFound();
    }

    DictionaryLookupResult transliterationResult = readEntry(connection.database(), transliterationQuery, word);
    if (transliterationResult.error.isError() || transliterationResult.found()) {
        return transliterationResult;
    }

    // Inflected surface forms ("went", "walked", "children") generated at build
    // time. A dictionary without the table, or one that predates it, keeps
    // working with exact lookup only.
    QSqlQuery formQuery(connection.database());
    formQuery.prepare(QStringLiteral("SELECT e.id, e.word, e.transliteration, e.pronunciation_respelling, "
                                     "e.pronunciation_ipa, e.source, e.license FROM word_forms f "
                                     "JOIN entries e ON e.id = f.entry_id WHERE f.form = ? COLLATE NOCASE "
                                     "ORDER BY e.id LIMIT 1"));
    formQuery.addBindValue(word);

    if (!formQuery.exec()) {
        qCDebug(lexiconDatabase) << "event=word_form_query_unavailable detail=" << formQuery.lastError().text();
        formQuery.finish();
        return DictionaryLookupResult::notFound();
    }

    return readEntry(connection.database(), formQuery, word, word);
}

DictionaryDescription SqliteDictionaryRepository::describe() const {
    if (m_databasePath.trimmed().isEmpty()) {
        return DictionaryDescription::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary database path is configured.")));
    }

    const QFileInfo databaseFile(m_databasePath);
    if (!databaseFile.exists() || !databaseFile.isFile()) {
        return DictionaryDescription::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("Dictionary database not found."), m_databasePath));
    }

    ReadOnlyConnection connection(m_databasePath, newConnectionName());
    if (!connection.open()) {
        const QString detail = connection.lastError();
        qCWarning(lexiconDatabase) << "event=database_open_failed path=" << m_databasePath << "detail=" << detail;
        return DictionaryDescription::failure(AppError::make(
            ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open the dictionary database."), detail));
    }

    QSqlQuery infoQuery(connection.database());
    infoQuery.prepare(QStringLiteral(
        "SELECT name, source_language, target_language, license, attribution FROM dictionary_info WHERE id = 1"));

    if (!infoQuery.exec()) {
        qCWarning(lexiconDatabase) << "event=dictionary_info_unavailable path=" << m_databasePath;
        return DictionaryDescription::failure(
            AppError::make(ErrorCode::DatabaseQueryFailed,
                           QStringLiteral("The file does not look like a Lexicon dictionary."), m_databasePath));
    }

    if (!infoQuery.next()) {
        return DictionaryDescription::failure(AppError::make(
            ErrorCode::DatabaseQueryFailed, QStringLiteral("The dictionary has no metadata row."), m_databasePath));
    }

    DictionaryDescription description;
    description.info.name = infoQuery.value(0).toString();
    description.info.sourceLanguage = normalizeLanguageTag(infoQuery.value(1).toString());
    description.info.targetLanguage = normalizeLanguageTag(infoQuery.value(2).toString());
    description.info.license = infoQuery.value(3).toString();
    description.info.attribution = infoQuery.value(4).toString();

    QSqlQuery countQuery(connection.database());
    if (countQuery.exec(QStringLiteral("SELECT COUNT(*) FROM entries")) && countQuery.next()) {
        description.entryCount = countQuery.value(0).toLongLong();
    } else {
        qCWarning(lexiconDatabase) << "event=entry_count_failed path=" << m_databasePath
                                   << "detail=" << countQuery.lastError().text();
    }

    return description;
}

QString SqliteDictionaryRepository::location() const {
    return m_databasePath;
}

} // namespace Lexicon

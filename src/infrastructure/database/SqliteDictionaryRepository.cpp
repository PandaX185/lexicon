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

SqliteDictionaryRepository::SqliteDictionaryRepository(QString databasePath)
    : m_databasePath(std::move(databasePath)) {}

DictionaryLookupResult SqliteDictionaryRepository::findByWord(const QString& word) const {
    if (m_databasePath.trimmed().isEmpty()) {
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary database path is configured.")));
    }

    const QFileInfo databaseFile(m_databasePath);
    if (!databaseFile.exists() || !databaseFile.isFile()) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::DatabaseUnavailable,
                           QStringLiteral("Dictionary database not found: %1").arg(m_databasePath), m_databasePath));
    }

    const QString connectionName =
        QStringLiteral("lexicon_lookup_%1").arg(QUuid::createUuid().toString(QUuid::WithoutBraces));

    const auto runLookup = [this, &word, &connectionName]() -> DictionaryLookupResult {
        QSqlDatabase database = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), connectionName);
        database.setDatabaseName(m_databasePath);
        database.setConnectOptions(QStringLiteral("QSQLITE_OPEN_READONLY"));

        if (!database.open()) {
            const QString detail = database.lastError().text();
            qCWarning(lexiconDatabase) << "event=database_open_failed path=" << m_databasePath << "detail=" << detail;
            return DictionaryLookupResult::failure(AppError::make(
                ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open the dictionary database."), detail));
        }

        QSqlQuery entryQuery(database);
        entryQuery.prepare(
            QStringLiteral("SELECT id, word, pronunciation_respelling, pronunciation_ipa, source, license "
                           "FROM entries WHERE word = ? COLLATE NOCASE LIMIT 1"));
        entryQuery.addBindValue(word);

        if (!entryQuery.exec()) {
            const QString detail = entryQuery.lastError().text();
            qCWarning(lexiconDatabase) << "event=entry_query_failed detail=" << detail;
            return DictionaryLookupResult::failure(AppError::make(
                ErrorCode::DatabaseQueryFailed, QStringLiteral("Could not query the dictionary database."), detail));
        }

        if (!entryQuery.next()) {
            return DictionaryLookupResult::notFound();
        }

        WordEntry entry;
        const qint64 entryId = entryQuery.value(0).toLongLong();
        entry.word = entryQuery.value(1).toString();
        entry.pronunciation.respelling = entryQuery.value(2).toString();
        entry.pronunciation.ipa = entryQuery.value(3).toString();

        const QString source = entryQuery.value(4).toString();
        const QString license = entryQuery.value(5).toString();
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
    };

    DictionaryLookupResult result = runLookup();
    QSqlDatabase::removeDatabase(connectionName);
    return result;
}

QString SqliteDictionaryRepository::location() const {
    return m_databasePath;
}

} // namespace Lexicon

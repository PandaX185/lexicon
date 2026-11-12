#pragma once

#include <QFile>
#include <QSqlDatabase>
#include <QSqlError>
#include <QSqlQuery>
#include <QString>
#include <QStringList>
#include <QUuid>

#include <algorithm>
#include <utility>

namespace Lexicon::Test {

class SqliteFixture {
  public:
    explicit SqliteFixture(QString databasePath)
        : m_databasePath(std::move(databasePath)),
          m_connectionName(QStringLiteral("lexicon_test_%1").arg(QUuid::createUuid().toString(QUuid::WithoutBraces))) {
        m_database = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), m_connectionName);
        m_database.setDatabaseName(m_databasePath);
        m_database.open();
    }

    ~SqliteFixture() {
        if (m_database.isOpen()) {
            m_database.close();
        }
        m_database = QSqlDatabase();
        QSqlDatabase::removeDatabase(m_connectionName);
    }

    SqliteFixture(const SqliteFixture&) = delete;
    SqliteFixture& operator=(const SqliteFixture&) = delete;

    [[nodiscard]] bool isOpen() const {
        return m_database.isOpen();
    }

    [[nodiscard]] QString lastError() const {
        return m_database.lastError().text();
    }

    [[nodiscard]] bool createSchema() {
        QFile schema(QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/schema.sql"));
        if (!schema.open(QIODevice::ReadOnly)) {
            return false;
        }

        const QString sql = QString::fromUtf8(schema.readAll());
        const QStringList statements = sql.split(QLatin1Char(';'), Qt::SkipEmptyParts);
        return std::ranges::all_of(
            statements, [this](const QString& statement) { return statement.trimmed().isEmpty() || exec(statement); });
    }

    [[nodiscard]] bool exec(const QString& sql) {
        QSqlQuery query(m_database);
        return query.exec(sql);
    }

    [[nodiscard]] QString path() const {
        return m_databasePath;
    }

    [[nodiscard]] QSqlDatabase& database() {
        return m_database;
    }

  private:
    QString m_databasePath;
    QString m_connectionName;
    QSqlDatabase m_database;
};

} // namespace Lexicon::Test

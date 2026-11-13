#include "app/AutostartManager.h"

#include "infrastructure/logging/Logging.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStandardPaths>

#include <utility>

namespace Lexicon {
namespace {

QString quoteExecArgument(const QString& value) {
    QString escaped = value;
    escaped.replace(QLatin1Char('\\'), QStringLiteral("\\\\"));
    escaped.replace(QLatin1Char('"'), QStringLiteral("\\\""));
    return QStringLiteral("\"%1\"").arg(escaped);
}

} // namespace

AutostartManager::AutostartManager(QString targetPath, QString templatePath)
    : m_targetPath(std::move(targetPath)), m_templatePath(std::move(templatePath)) {}

AutostartManager AutostartManager::forCurrentUser() {
    const QString target = QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation) +
                           QStringLiteral("/autostart/%1.desktop").arg(QStringLiteral(LEXICON_APP_ID));
    const QStringList candidates = {
        QCoreApplication::applicationDirPath() + QStringLiteral("/../share/lexicon/lexicon-autostart.desktop"),
        QStringLiteral("/usr/share/lexicon/lexicon-autostart.desktop"),
        QStringLiteral("/usr/local/share/lexicon/lexicon-autostart.desktop"),
    };
    for (const QString& path : candidates) {
        if (QFile::exists(path)) {
            return {target, path};
        }
    }
    return {target, {}};
}

bool AutostartManager::install(const QString& executablePath) const {
    if (m_templatePath.isEmpty()) {
        qCWarning(lexiconApp) << "event=autostart_install_failed reason=template_missing";
        return false;
    }

    QFile source(m_templatePath);
    if (!source.open(QIODevice::ReadOnly | QIODevice::Text)) {
        qCWarning(lexiconApp) << "event=autostart_install_failed reason=open_failed path=" << m_templatePath;
        return false;
    }
    QString content = QString::fromUtf8(source.readAll());
    source.close();

    // The login session may not have the same PATH as the shell that installed
    // the application, so write the absolute path into the entry.
    const QString execLine = QStringLiteral("Exec=%1").arg(quoteExecArgument(executablePath));
    content.replace(QRegularExpression(QStringLiteral("^Exec=.*$"), QRegularExpression::MultilineOption), execLine);

    QDir().mkpath(QFileInfo(m_targetPath).absolutePath());
    QSaveFile out(m_targetPath);
    if (!out.open(QIODevice::WriteOnly | QIODevice::Text)) {
        qCWarning(lexiconApp) << "event=autostart_install_failed reason=write_failed path=" << m_targetPath;
        return false;
    }
    if (out.write(content.toUtf8()) < 0 || !out.commit()) {
        qCWarning(lexiconApp) << "event=autostart_install_failed reason=write_failed path=" << m_targetPath
                              << "error=" << out.errorString();
        return false;
    }
    qCInfo(lexiconApp) << "event=autostart_installed path=" << m_targetPath;
    return true;
}

bool AutostartManager::remove() const {
    if (!QFile::exists(m_targetPath)) {
        return true;
    }
    if (QFile::remove(m_targetPath)) {
        qCInfo(lexiconApp) << "event=autostart_removed path=" << m_targetPath;
        return true;
    }
    qCWarning(lexiconApp) << "event=autostart_remove_failed path=" << m_targetPath;
    return false;
}

} // namespace Lexicon

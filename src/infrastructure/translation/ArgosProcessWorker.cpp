#include "infrastructure/translation/ArgosProcessWorker.h"

#include "infrastructure/logging/Logging.h"

#include <QDeadlineTimer>
#include <QJsonDocument>
#include <QJsonParseError>
#include <QStandardPaths>

#include <algorithm>
#include <utility>

namespace Lexicon {
namespace {

constexpr int ProtocolVersion = 1;

ErrorCode errorCodeFromWorker(const QString& code) {
    if (code == QStringLiteral("invalid_argument")) {
        return ErrorCode::InvalidArgument;
    }
    if (code == QStringLiteral("pair_not_installed") || code == QStringLiteral("runtime_missing")) {
        return ErrorCode::TranslationUnavailable;
    }
    return ErrorCode::Unknown;
}

} // namespace

QString ArgosProcessWorker::scriptPath() {
    QString overridePath = qEnvironmentVariable("LEXICON_ARGOS_WORKER");
    if (!overridePath.isEmpty()) {
        return overridePath;
    }
    return QStandardPaths::locate(QStandardPaths::GenericDataLocation,
                                  QStringLiteral("lexicon/scripts/argos_worker.py"));
}

QStringList ArgosProcessWorker::defaultArguments() {
    const QString script = scriptPath();
    if (script.isEmpty()) {
        return {};
    }
    return {script};
}

ArgosProcessWorker::ArgosProcessWorker(QString program, QStringList arguments)
    : m_program(std::move(program)), m_arguments(std::move(arguments)) {}

ArgosProcessWorker::~ArgosProcessWorker() {
    stop();
}

ServiceStatus ArgosProcessWorker::start() {
    const std::scoped_lock lock(m_mutex);
    return startLocked();
}

ServiceStatus ArgosProcessWorker::startLocked() {
    if (isRunningLocked()) {
        return ServiceStatus::success();
    }

    if (m_arguments.isEmpty()) {
        return ServiceStatus::failure(AppError::make(
            ErrorCode::TranslationUnavailable, QStringLiteral("The Argos worker script was not found."),
            QStringLiteral("Install the translation component, or set LEXICON_ARGOS_WORKER to the worker script.")));
    }

    m_buffer.clear();
    m_process = std::make_unique<QProcess>();
    m_process->setProgram(m_program);
    m_process->setArguments(m_arguments);
    m_process->start();

    if (!m_process->waitForStarted(3000)) {
        const QString detail = m_process->errorString();
        m_process = nullptr;
        return ServiceStatus::failure(AppError::make(ErrorCode::TranslationUnavailable,
                                                     QStringLiteral("The Argos worker could not be started."), detail));
    }

    qCInfo(lexiconApp) << "event=argos_worker_started program=" << m_program;
    return ServiceStatus::success();
}

void ArgosProcessWorker::stop() {
    const std::scoped_lock lock(m_mutex);
    stopLocked();
}

void ArgosProcessWorker::stopLocked() {
    if (m_process) {
        if (m_process->state() != QProcess::NotRunning) {
            m_process->terminate();
            if (!m_process->waitForFinished(1000)) {
                m_process->kill();
                m_process->waitForFinished(500);
            }
        }
        m_process = nullptr;
    }
    m_buffer.clear();
}

bool ArgosProcessWorker::isRunning() {
    const std::scoped_lock lock(m_mutex);
    return isRunningLocked();
}

bool ArgosProcessWorker::isRunningLocked() const {
    return m_process && m_process->state() == QProcess::Running;
}

void ArgosProcessWorker::drainStderrLocked() {
    if (!m_process) {
        return;
    }
    const QByteArray diagnostics = m_process->readAllStandardError().trimmed();
    if (!diagnostics.isEmpty()) {
        // Only the worker's own diagnostics are logged; request text never is, so
        // selected words do not reach the journal.
        qCDebug(lexiconApp) << "event=argos_worker_stderr output=" << diagnostics;
    }
}

QByteArray ArgosProcessWorker::readLineLocked(int timeoutMs) {
    QDeadlineTimer deadline(timeoutMs);

    for (;;) {
        // Buffered output is drained first: a short-lived process may have
        // exited before its answer was read.
        if (m_process) {
            m_buffer.append(m_process->readAllStandardOutput());
        }

        const qsizetype newline = m_buffer.indexOf('\n');
        if (newline >= 0) {
            const QByteArray line = m_buffer.left(newline).trimmed();
            m_buffer.remove(0, newline + 1);
            return line;
        }

        if (!m_process || m_process->state() == QProcess::NotRunning) {
            return {};
        }

        const qint64 remaining = deadline.remainingTime();
        if (remaining == 0) {
            return {};
        }
        const int waitMs = remaining < 0 ? timeoutMs : static_cast<int>(std::max<qint64>(1, remaining));

        m_process->waitForReadyRead(waitMs);
        drainStderrLocked();
    }
}

Result<QJsonObject> ArgosProcessWorker::sendRequest(const QString& action, const QJsonObject& params, int timeoutMs) {
    const std::scoped_lock lock(m_mutex);

    if (!isRunningLocked()) {
        const ServiceStatus status = startLocked();
        if (!status.available) {
            return status.error;
        }
    }

    const int id = ++m_requestId;
    QJsonObject request{
        {QStringLiteral("id"), id},
        {QStringLiteral("version"), ProtocolVersion},
        {QStringLiteral("action"), action},
        {QStringLiteral("params"), params},
    };

    const QByteArray payload = QJsonDocument(request).toJson(QJsonDocument::Compact) + '\n';
    if (m_process->write(payload) != payload.size()) {
        stopLocked();
        return AppError::make(ErrorCode::TranslationUnavailable,
                              QStringLiteral("The Argos worker did not accept the request."));
    }
    // A worker that answers and exits immediately may already be gone here, so a
    // failed flush is not an error yet: the answer is read below.
    m_process->waitForBytesWritten(timeoutMs);

    const QByteArray line = readLineLocked(timeoutMs);
    drainStderrLocked();
    if (line.isEmpty()) {
        const bool exited = !m_process || m_process->state() == QProcess::NotRunning;
        stopLocked();
        if (exited) {
            return AppError::make(ErrorCode::TranslationUnavailable,
                                  QStringLiteral("The Argos worker exited before answering."));
        }
        return AppError::make(ErrorCode::Unknown, QStringLiteral("The Argos worker timed out answering."));
    }

    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(line, &parseError);
    if (document.isNull() || !document.isObject()) {
        return AppError::make(ErrorCode::ProtocolViolation,
                              QStringLiteral("The Argos worker sent a malformed response."), QString::fromUtf8(line));
    }

    const QJsonObject response = document.object();
    if (!response.value(QStringLiteral("success")).toBool()) {
        const QJsonObject error = response.value(QStringLiteral("error")).toObject();
        return AppError::make(errorCodeFromWorker(error.value(QStringLiteral("code")).toString()),
                              error.value(QStringLiteral("message")).toString());
    }

    return response.value(QStringLiteral("result")).toObject();
}

} // namespace Lexicon

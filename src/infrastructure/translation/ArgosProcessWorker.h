#pragma once

#include "core/types/Result.h"
#include "core/types/ServiceStatus.h"

#include <QByteArray>
#include <QJsonObject>
#include <QProcess>
#include <QString>
#include <QStringList>

#include <memory>
#include <mutex>

namespace Lexicon {

// Runs the optional Argos Translate worker: a local Python process speaking
// newline-delimited JSON over stdin/stdout. Requests are synchronous, so this is
// called from the lookup worker thread, never from the GUI thread, and the
// worker never fetches anything by itself.
class ArgosProcessWorker {
  public:
    // The bundled worker script, or an override from LEXICON_ARGOS_WORKER.
    [[nodiscard]] static QString scriptPath();
    [[nodiscard]] static QStringList defaultArguments();

    explicit ArgosProcessWorker(QString program = QStringLiteral("python3"), QStringList arguments = {});
    ~ArgosProcessWorker();

    ArgosProcessWorker(const ArgosProcessWorker&) = delete;
    ArgosProcessWorker& operator=(const ArgosProcessWorker&) = delete;

    ServiceStatus start();
    void stop();
    [[nodiscard]] bool isRunning();

    [[nodiscard]] Result<QJsonObject> sendRequest(const QString& action, const QJsonObject& params,
                                                  int timeoutMs = 5000);

  private:
    ServiceStatus startLocked();
    void stopLocked();
    [[nodiscard]] bool isRunningLocked() const;
    [[nodiscard]] QByteArray readLineLocked(int timeoutMs);
    void drainStderrLocked();

    QString m_program;
    QStringList m_arguments;
    std::unique_ptr<QProcess> m_process;
    QByteArray m_buffer;
    int m_requestId = 0;
    std::mutex m_mutex;
};

} // namespace Lexicon

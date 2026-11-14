#pragma once

#include <QString>
#include <QStringList>

namespace Lexicon {

/// Directory where local crash reports are written. Uses the user's state
/// directory (XDG_STATE_HOME on Linux) and never the network.
[[nodiscard]] QString crashReportDirectory();

/// Render a crash report. `frames` are backtrace symbols; nothing else from the
/// process (selected text, documents, environment) is included.
[[nodiscard]] QString formatCrashReport(int signalNumber, const QStringList& frames, qint64 processId);

/// Write a report into `directory`, creating it if needed. Returns the report
/// path, or an empty string when the report could not be written.
[[nodiscard]] QString writeCrashReport(const QString& directory, int signalNumber, const QStringList& frames,
                                       qint64 processId);

/// Install handlers for fatal POSIX signals. Each crash writes one private
/// report into `directory` using only async-signal-safe operations, then lets
/// the default handler terminate the process. Call once, before starting the
/// main loop.
void installCrashHandler(const QString& directory);

} // namespace Lexicon

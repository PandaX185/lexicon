#include "infrastructure/CrashReporter.h"

#include "infrastructure/logging/Logging.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QStandardPaths>

#include <array>
#include <csignal>
#include <cstring>
#include <execinfo.h>
#include <fcntl.h>
#include <unistd.h>

namespace {

constexpr std::size_t MaxFrames = 64;
constexpr std::size_t MaxPathLength = 4096;
// Room for the process ID, ".log", and the terminator appended to the prefix.
constexpr std::size_t PathSuffixReserve = 32;

// Built once in installCrashHandler(), because the signal handler cannot
// allocate or touch Qt.
std::array<char, MaxPathLength> g_reportPrefix{};
volatile sig_atomic_t g_handling = 0;

const char* signalName(int signalNumber) {
    switch (signalNumber) {
    case SIGSEGV:
        return "SIGSEGV";
    case SIGABRT:
        return "SIGABRT";
    case SIGBUS:
        return "SIGBUS";
    case SIGILL:
        return "SIGILL";
    case SIGFPE:
        return "SIGFPE";
    default:
        return "UNKNOWN";
    }
}

char* appendText(char* cursor, const char* text) {
    while (*text != '\0') {
        *cursor++ = *text++;
    }
    return cursor;
}

char* appendNumber(char* cursor, long value) {
    if (value == 0) {
        *cursor++ = '0';
        return cursor;
    }

    std::array<char, 32> digits{};
    std::size_t length = 0;
    unsigned long magnitude = value < 0 ? static_cast<unsigned long>(-value) : static_cast<unsigned long>(value);
    if (value < 0) {
        *cursor++ = '-';
    }
    while (magnitude > 0) {
        digits[length++] = static_cast<char>('0' + (magnitude % 10));
        magnitude /= 10;
    }
    while (length > 0) {
        *cursor++ = digits[--length];
    }
    return cursor;
}

// Async-signal-safe replacement for formatting the fixed report header.
void writeHeader(int fd, int signalNumber) {
    std::array<char, 256> buffer{};
    char* cursor = appendText(buffer.data(), "Lexicon crash report\nVersion: " LEXICON_VERSION "\nSignal: ");
    cursor = appendText(cursor, signalName(signalNumber));
    cursor = appendText(cursor, " (");
    cursor = appendNumber(cursor, signalNumber);
    cursor = appendText(cursor, ")\nProcess: ");
    cursor = appendNumber(cursor, static_cast<long>(getpid()));
    cursor = appendText(cursor, "\nFrames:\n");
    ::write(fd, buffer.data(), static_cast<std::size_t>(cursor - buffer.data()));
}

void crashSignalHandler(int signalNumber) {
    if (g_handling != 0) {
        ::signal(signalNumber, SIG_DFL);
        ::raise(signalNumber);
        return;
    }
    g_handling = 1;

    std::array<char, MaxPathLength> path{};
    char* cursor = appendText(path.data(), g_reportPrefix.data());
    cursor = appendNumber(cursor, static_cast<long>(getpid()));
    cursor = appendText(cursor, ".log");
    *cursor = '\0';

    const int fd = ::open(path.data(), O_WRONLY | O_CREAT | O_TRUNC, 0600);
    if (fd >= 0) {
        writeHeader(fd, signalNumber);

        std::array<void*, MaxFrames> frames{};
        const int count = ::backtrace(frames.data(), static_cast<int>(frames.size()));
        if (count > 0) {
            ::backtrace_symbols_fd(frames.data(), count, fd);
        }
        ::close(fd);
    }

    // Preserve the normal crash path so system crash tooling still sees the
    // fatal signal instead of a silently exiting process.
    struct sigaction action{};
    action.sa_handler = SIG_DFL;
    sigemptyset(&action.sa_mask);
    ::sigaction(signalNumber, &action, nullptr);
    ::raise(signalNumber);
}

} // namespace

namespace Lexicon {

QString crashReportDirectory() {
    QString base = QStandardPaths::writableLocation(QStandardPaths::GenericStateLocation);
    if (base.isEmpty()) {
        base = QStandardPaths::writableLocation(QStandardPaths::AppLocalDataLocation);
    }
    if (base.isEmpty()) {
        base = QDir::homePath() + QStringLiteral("/.local/state");
    }
    return QDir(base).filePath(QStringLiteral("lexicon"));
}

QString formatCrashReport(int signalNumber, const QStringList& frames, qint64 processId) {
    QString report;
    report += QStringLiteral("Lexicon crash report\n");
    report += QStringLiteral("Version: %1\n").arg(QStringLiteral(LEXICON_VERSION));
    report += QStringLiteral("Signal: %1 (%2)\n").arg(QString::fromLatin1(signalName(signalNumber))).arg(signalNumber);
    report += QStringLiteral("Process: %1\n").arg(processId);
    report += QStringLiteral("Frames:\n");

    for (qsizetype index = 0; index < frames.size(); ++index) {
        report += QStringLiteral("  #%1 %2\n").arg(index).arg(frames.at(index));
    }
    return report;
}

QString writeCrashReport(const QString& directory, int signalNumber, const QStringList& frames, qint64 processId) {
    if (!QDir().mkpath(directory)) {
        return {};
    }

    const QString stamp = QDateTime::currentDateTimeUtc().toString(QStringLiteral("yyyyMMdd'T'HHmmss'Z'"));

    QFile file(QDir(directory).filePath(QStringLiteral("lexicon-crash-%1-%2.log").arg(stamp).arg(processId)));
    if (!file.open(QIODevice::WriteOnly | QIODevice::NewOnly)) {
        return {};
    }
    QFile::setPermissions(file.fileName(), QFileDevice::ReadOwner | QFileDevice::WriteOwner);

    file.write(formatCrashReport(signalNumber, frames, processId).toUtf8());
    file.close();
    return file.fileName();
}

void installCrashHandler(const QString& directory) {
    if (!QDir().mkpath(directory)) {
        qCWarning(lexiconApp) << "event=crash_reporter_unavailable directory=" << directory;
        return;
    }

    const QByteArray prefix = QFile::encodeName(QDir(directory).filePath(QStringLiteral("lexicon-crash-")));
    if (prefix.size() + static_cast<qsizetype>(PathSuffixReserve) >= static_cast<qsizetype>(MaxPathLength)) {
        qCWarning(lexiconApp) << "event=crash_reporter_unavailable reason=path_too_long";
        return;
    }

    std::memcpy(g_reportPrefix.data(), prefix.constData(), static_cast<std::size_t>(prefix.size()));
    g_reportPrefix.at(static_cast<std::size_t>(prefix.size())) = '\0';

    // Warm up the unwinder so the handler does not need to load libgcc lazily
    // while the process is already crashing.
    std::array<void*, 1> warmup{};
    ::backtrace(warmup.data(), 1);

    struct sigaction action{};
    action.sa_handler = crashSignalHandler;
    sigemptyset(&action.sa_mask);
    action.sa_flags = SA_RESETHAND;

    for (const int signalNumber : {SIGSEGV, SIGABRT, SIGBUS, SIGILL, SIGFPE}) {
        ::sigaction(signalNumber, &action, nullptr);
    }

    qCInfo(lexiconApp) << "event=crash_reporter_installed directory=" << directory;
}

} // namespace Lexicon

#include "infrastructure/CrashReporter.h"
#include "support/ScopedEnvironment.h"

#include <QDir>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>

#include <csignal>
#include <sys/wait.h>
#include <unistd.h>

using namespace Lexicon;
using Lexicon::Test::ScopedEnvironment;

class CrashReporterTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void usesXdgStateDirectory();
    void formatIncludesSignalAndFrames();
    void formatOmitsSelectedText();
    void writesPrivateReportFile();
    void handlerWritesReportWhenProcessCrashes();
};

void CrashReporterTest::usesXdgStateDirectory() {
    QTemporaryDir state;
    QVERIFY(state.isValid());
    ScopedEnvironment env("XDG_STATE_HOME", state.path().toLocal8Bit());

    QCOMPARE(crashReportDirectory(), QDir(state.path()).filePath(QStringLiteral("lexicon")));
}

void CrashReporterTest::formatIncludesSignalAndFrames() {
    const QString report = formatCrashReport(SIGSEGV, {QStringLiteral("lexicon(+0x1234) [0x00001234]")}, 4242);

    QVERIFY(report.contains(QStringLiteral("Version: " LEXICON_VERSION)));
    QVERIFY(report.contains(QStringLiteral("SIGSEGV")));
    QVERIFY(report.contains(QStringLiteral("4242")));
    QVERIFY(report.contains(QStringLiteral("lexicon(+0x1234) [0x00001234]")));
}

void CrashReporterTest::formatOmitsSelectedText() {
    // Crash reports are local diagnostics only; they must not include the text
    // the user looked up or any other process data.
    ScopedEnvironment env("LEXICON_SELECTED_TEXT", QByteArrayLiteral("top-secret-selection"));

    const QString report = formatCrashReport(SIGABRT, {QStringLiteral("#0 main")}, 1);

    QVERIFY(!report.contains(QStringLiteral("top-secret-selection")));
    QVERIFY(!report.contains(QStringLiteral("LEXICON_SELECTED_TEXT")));
}

void CrashReporterTest::writesPrivateReportFile() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const QString path = writeCrashReport(directory.path(), SIGABRT, {QStringLiteral("#0 main")}, 777);
    QVERIFY(!path.isEmpty());
    QVERIFY(QFile::exists(path));

    QFile file(path);
    QVERIFY(file.open(QIODevice::ReadOnly));
    const QByteArray contents = file.readAll();
    QVERIFY(contents.contains("SIGABRT"));
    QVERIFY(contents.contains("#0 main"));
    QVERIFY(contents.contains("777"));

    const QFileDevice::Permissions permissions = QFile::permissions(path);
    QVERIFY(permissions.testFlag(QFileDevice::ReadOwner));
    QVERIFY(!permissions.testFlag(QFileDevice::ReadGroup));
    QVERIFY(!permissions.testFlag(QFileDevice::ReadOther));
}

void CrashReporterTest::handlerWritesReportWhenProcessCrashes() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const pid_t pid = fork();
    QVERIFY(pid >= 0);
    if (pid == 0) {
        installCrashHandler(directory.path());
        raise(SIGSEGV);
        _exit(1);
    }

    int status = 0;
    QCOMPARE(waitpid(pid, &status, 0), pid);
    QVERIFY(WIFSIGNALED(status));
    QCOMPARE(WTERMSIG(status), SIGSEGV);

    const QStringList reports = QDir(directory.path()).entryList(QStringList{QStringLiteral("*.log")}, QDir::Files);
    QCOMPARE(reports.size(), 1);

    QFile file(QDir(directory.path()).filePath(reports.constFirst()));
    QVERIFY(file.open(QIODevice::ReadOnly));
    const QByteArray contents = file.readAll();
    QVERIFY(contents.contains("Version: " LEXICON_VERSION));
    QVERIFY(contents.contains("SIGSEGV"));
    QVERIFY(contents.contains(QByteArray::number(static_cast<qint64>(pid))));
}

QTEST_GUILESS_MAIN(CrashReporterTest)

#include "test_crash_reporter.moc"

#include "infrastructure/translation/ArgosProcessWorker.h"

#include <QFileInfo>
#include <QJsonArray>
#include <QJsonObject>
#include <QStandardPaths>
#include <QTest>

using namespace Lexicon;

namespace {

QString workerScript() {
    return QStringLiteral(LEXICON_SOURCE_DIR "/scripts/argos_worker.py");
}

bool workerAvailable() {
    return !QStandardPaths::findExecutable(QStringLiteral("python3")).isEmpty() && QFileInfo::exists(workerScript());
}

} // namespace

class ArgosWorkerProtocolTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void repliesToPing();
    void reportsInstalledPairs();
    void reportsMissingLanguagePair();
    void rejectsMalformedResponse();
    void timesOutOnSilentWorker();
    void reusesOneProcessForSeveralRequests();
};

void ArgosWorkerProtocolTest::repliesToPing() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosProcessWorker worker(QStringLiteral("python3"), {workerScript()});
    QVERIFY(worker.start().available);

    const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("ping"), {});
    QVERIFY(response.hasValue());
    QCOMPARE(response->value(QStringLiteral("status")).toString(), QStringLiteral("ok"));
    QCOMPARE(response->value(QStringLiteral("version")).toInt(), 1);
}

void ArgosWorkerProtocolTest::reportsInstalledPairs() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosProcessWorker worker(QStringLiteral("python3"), {workerScript()});
    const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("list_installed_pairs"), {});
    QVERIFY(response.hasValue());
    // Without argostranslate installed the answer is an empty list, not an error.
    QVERIFY(response->value(QStringLiteral("pairs")).isArray());
}

void ArgosWorkerProtocolTest::reportsMissingLanguagePair() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosProcessWorker worker(QStringLiteral("python3"), {workerScript()});
    const QJsonObject params{
        {QStringLiteral("text"), QStringLiteral("hello")},
        {QStringLiteral("from"), QStringLiteral("en")},
        {QStringLiteral("to"), QStringLiteral("zz")},
    };
    const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("translate"), params);

    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::TranslationUnavailable);
    QVERIFY(response.error().message.contains(QStringLiteral("not installed")));
}

void ArgosWorkerProtocolTest::rejectsMalformedResponse() {
    ArgosProcessWorker worker(QStringLiteral("/bin/echo"), {QStringLiteral("NOT JSON")});
    const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("ping"), {}, 2000);

    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::ProtocolViolation);
}

void ArgosWorkerProtocolTest::timesOutOnSilentWorker() {
    ArgosProcessWorker worker(QStringLiteral("/bin/sleep"), {QStringLiteral("10")});
    const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("ping"), {}, 300);

    QVERIFY(!response.hasValue());
    QCOMPARE(response.error().code, ErrorCode::Unknown);
    QVERIFY(!worker.isRunning());
}

void ArgosWorkerProtocolTest::reusesOneProcessForSeveralRequests() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosProcessWorker worker(QStringLiteral("python3"), {workerScript()});
    for (int i = 0; i < 3; ++i) {
        const Result<QJsonObject> response = worker.sendRequest(QStringLiteral("ping"), {});
        QVERIFY(response.hasValue());
        QCOMPARE(response->value(QStringLiteral("status")).toString(), QStringLiteral("ok"));
    }
    QVERIFY(worker.isRunning());
}

QTEST_GUILESS_MAIN(ArgosWorkerProtocolTest)

#include "test_argos_worker_protocol.moc"

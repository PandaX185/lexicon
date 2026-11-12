#include "core/selection/PrimarySelectionProvider.h"
#include "core/selection/SelectionTextPolicy.h"
#include "platform/wayland/WaylandDataControlSource.h"

#include <QProcess>
#include <QStandardPaths>
#include <QTest>

#include <functional>
#include <memory>

using namespace Lexicon;

/// Live-session tests for the Wayland primary-selection path.
///
/// These set the real primary selection with wl-copy, so they are labelled
/// "kde" and excluded from the default/CI presets. Run with:
///
///     ctest --test-dir build/dev-debug -L kde --output-on-failure
class WaylandSelectionLiveTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void initTestCase();
    void readsPrimarySelection();
    void reportsNoSelection();
    void reportsNonTextSelection();
    void reportsTooLongSelection();
    void cleanupTestCase();

  private:
    static bool runWlCopy(const QStringList& arguments, const QByteArray& data = {});
    static SelectionResult readOnce();
    static SelectionResult readUntil(const std::function<bool(const SelectionResult&)>& predicate);
};

void WaylandSelectionLiveTest::initTestCase() {
    if (qEnvironmentVariableIsEmpty("WAYLAND_DISPLAY")) {
        QSKIP("Not running in a Wayland session.");
    }
    if (QStandardPaths::findExecutable(QStringLiteral("wl-copy")).isEmpty()) {
        QSKIP("wl-copy is required to set the primary selection for live tests.");
    }
}

bool WaylandSelectionLiveTest::runWlCopy(const QStringList& arguments, const QByteArray& data) {
    QProcess process;
    process.start(QStringLiteral("wl-copy"), QStringList{QStringLiteral("--primary")} + arguments);
    if (!process.waitForStarted(5000)) {
        return false;
    }
    if (!data.isEmpty()) {
        process.write(data);
        process.closeWriteChannel();
    }
    return process.waitForFinished(5000) && process.exitStatus() == QProcess::NormalExit && process.exitCode() == 0;
}

SelectionResult WaylandSelectionLiveTest::readOnce() {
    PrimarySelectionProvider provider(std::make_unique<WaylandDataControlSource>());
    return provider.currentSelection();
}

SelectionResult WaylandSelectionLiveTest::readUntil(const std::function<bool(const SelectionResult&)>& predicate) {
    SelectionResult result = readOnce();
    for (int attempt = 0; attempt < 20 && !predicate(result); ++attempt) {
        QTest::qWait(50);
        result = readOnce();
    }
    return result;
}

void WaylandSelectionLiveTest::readsPrimarySelection() {
    QVERIFY(runWlCopy({}, QByteArray("serendipity")));

    const SelectionResult result = readUntil([](const SelectionResult& current) { return current.hasSelection(); });

    QVERIFY(result.hasSelection());
    QCOMPARE(*result.text, QStringLiteral("serendipity"));
}

void WaylandSelectionLiveTest::reportsNoSelection() {
    QVERIFY(runWlCopy({QStringLiteral("--clear")}));

    const SelectionResult result =
        readUntil([](const SelectionResult& current) { return current.error.code == ErrorCode::SelectionUnavailable; });

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnavailable);
}

void WaylandSelectionLiveTest::reportsNonTextSelection() {
    QVERIFY(runWlCopy({QStringLiteral("--type"), QStringLiteral("application/x-lexicon-test")}, QByteArray("binary")));

    const SelectionResult result =
        readUntil([](const SelectionResult& current) { return current.error.code == ErrorCode::SelectionNotText; });

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionNotText);
}

void WaylandSelectionLiveTest::reportsTooLongSelection() {
    QVERIFY(runWlCopy({}, QByteArray(129, 'a')));

    const SelectionResult result =
        readUntil([](const SelectionResult& current) { return current.error.code == ErrorCode::SelectionTooLong; });

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionTooLong);
}

void WaylandSelectionLiveTest::cleanupTestCase() {
    runWlCopy({QStringLiteral("--clear")});
}

QTEST_GUILESS_MAIN(WaylandSelectionLiveTest)

#include "test_wayland_selection.moc"

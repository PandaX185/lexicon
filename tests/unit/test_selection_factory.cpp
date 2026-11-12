#include "app/SelectionProviderFactory.h"
#include "core/interfaces/SelectionProvider.h"
#include "support/ScopedEnvironment.h"

#include <QTest>

#include <memory>

using namespace Lexicon;
using namespace Lexicon::Test;

namespace {

std::unique_ptr<SelectionProvider> providerFor(const QByteArray& sessionType, const QByteArray& waylandDisplay,
                                               const QByteArray& display) {
    ScopedEnvironment session("XDG_SESSION_TYPE", sessionType);
    ScopedEnvironment wayland("WAYLAND_DISPLAY", waylandDisplay);
    ScopedEnvironment x11("DISPLAY", display);
    return createSelectionProvider();
}

} // namespace

/// The factory is the only platform-aware code. These tests pin the
/// degradation contract: every session without a working backend must report
/// an explicit, distinct error and must never silently read the clipboard.
class SelectionFactoryTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void x11ReportsUnsupported();
    void noDisplayReportsUnsupported();
    void waylandSessionSelectsWaylandProvider();
};

void SelectionFactoryTest::x11ReportsUnsupported() {
    const auto provider = providerFor("x11", {}, ":0");

    QVERIFY(provider->displayName().contains(QStringLiteral("X11")));

    const SelectionResult result = provider->currentSelection();
    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnsupported);
    QVERIFY(result.error.detail.contains(QStringLiteral("clipboard")));
}

void SelectionFactoryTest::noDisplayReportsUnsupported() {
    const auto provider = providerFor({}, {}, {});

    const SelectionResult result = provider->currentSelection();
    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnsupported);
}

void SelectionFactoryTest::waylandSessionSelectsWaylandProvider() {
    const auto provider = providerFor("wayland", {}, {});

    // Depending on the build, this is either the real Wayland backend or the
    // explicit "backend not built" fallback. Both must be clearly identified
    // and must never claim to be X11.
    QVERIFY(provider->displayName().contains(QStringLiteral("Wayland")));
}

QTEST_GUILESS_MAIN(SelectionFactoryTest)

#include "test_selection_factory.moc"

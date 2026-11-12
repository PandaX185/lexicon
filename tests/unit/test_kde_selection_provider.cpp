#include "kde/selection/KdeSelectionProvider.h"

#include <QTest>

using namespace Lexicon;

class KdeSelectionProviderTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void returnsExplicitUnavailableResult();
    void exposesDisplayName();
};

void KdeSelectionProviderTest::returnsExplicitUnavailableResult() {
    KdeSelectionProvider provider;

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QVERIFY(result.error.isError());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnavailable);
    QVERIFY(!result.error.message.isEmpty());
    QVERIFY(!result.error.detail.isEmpty());
}

void KdeSelectionProviderTest::exposesDisplayName() {
    KdeSelectionProvider provider;

    QVERIFY(!provider.displayName().isEmpty());
}

QTEST_GUILESS_MAIN(KdeSelectionProviderTest)

#include "test_kde_selection_provider.moc"

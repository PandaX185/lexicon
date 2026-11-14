#include "app/PopupPlacement.h"

#include <QTest>

using namespace Lexicon;

/// Placement is expressed in the same logical (device-independent) coordinates
/// Qt uses for screens, windows, and the cursor, so the same rules cover
/// multi-monitor layouts and high-DPI scaling.
class PopupPlacementTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void centersOnScreen();
    void placesNearCursor();
    void clampsAtScreenEdges();
    void clampsOnNegativeOriginMonitor();
    void acceptsExactFit();
    void rejectsPopupLargerThanScreen();
};

void PopupPlacementTest::centersOnScreen() {
    const QRect available(0, 0, 1920, 1080);
    const QSize popup(360, 240);

    const std::optional<QPoint> position = placePopup(available, QPoint(100, 100), popup, PopupPosition::Center);

    QVERIFY(position.has_value());
    QCOMPARE(*position, QPoint(779, 419));
}

void PopupPlacementTest::placesNearCursor() {
    const QRect available(0, 0, 1920, 1080);

    for (const PopupPosition mode : {PopupPosition::NearSelection, PopupPosition::Cursor}) {
        const std::optional<QPoint> position = placePopup(available, QPoint(100, 100), QSize(360, 240), mode);

        QVERIFY(position.has_value());
        QCOMPARE(*position, QPoint(116, 116));
    }
}

void PopupPlacementTest::clampsAtScreenEdges() {
    const QRect available(0, 0, 1920, 1080);

    const std::optional<QPoint> position =
        placePopup(available, QPoint(1900, 1070), QSize(360, 240), PopupPosition::NearSelection);

    QVERIFY(position.has_value());
    QCOMPARE(*position, QPoint(1559, 839));
}

void PopupPlacementTest::clampsOnNegativeOriginMonitor() {
    const QRect leftMonitor(-2560, 0, 2560, 1440);

    const std::optional<QPoint> position =
        placePopup(leftMonitor, QPoint(-10, 1400), QSize(400, 300), PopupPosition::NearSelection);

    QVERIFY(position.has_value());
    QCOMPARE(*position, QPoint(-401, 1139));
}

void PopupPlacementTest::acceptsExactFit() {
    const QRect available(0, 0, 200, 100);

    const std::optional<QPoint> position =
        placePopup(available, QPoint(0, 0), QSize(200, 100), PopupPosition::NearSelection);

    QVERIFY(position.has_value());
    QCOMPARE(*position, QPoint(0, 0));
}

void PopupPlacementTest::rejectsPopupLargerThanScreen() {
    const QRect available(0, 0, 200, 100);

    QVERIFY(!placePopup(available, QPoint(0, 0), QSize(300, 200), PopupPosition::Center).has_value());
    QVERIFY(!placePopup(available, QPoint(0, 0), QSize(100, 200), PopupPosition::Center).has_value());
}

QTEST_GUILESS_MAIN(PopupPlacementTest)

#include "test_popup_placement.moc"

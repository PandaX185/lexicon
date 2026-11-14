#include "app/SettingsController.h"
#include "config/Config.h"

#include <QAccessible>
#include <QColor>
#include <QQmlApplicationEngine>
#include <QQmlComponent>
#include <QQmlContext>
#include <QQmlEngine>
#include <QQuickItem>
#include <QQuickView>
#include <QQuickWindow>
#include <QTest>
#include <QUrl>

#include <memory>

using namespace Lexicon;

namespace {

QUrl sourceUrl(const QString& relativePath) {
    return QUrl::fromLocalFile(QStringLiteral(LEXICON_SOURCE_DIR "/") + relativePath);
}

QVariantMap qmlConfig(bool darkMode = true) {
    Config config;
    config.appearance.theme = darkMode ? Theme::Dark : Theme::Light;
    return config.toVariantMap(darkMode ? Qt::ColorScheme::Dark : Qt::ColorScheme::Light);
}

// The real object is a QObject created by the application; the view only needs
// the properties it binds to.
QVariantMap qmlLanguagePacks() {
    return QVariantMap{
        {QStringLiteral("status"), QStringLiteral("No language pack catalog is configured.")},
        {QStringLiteral("installedPairs"), QVariantList{}},
        {QStringLiteral("availablePacks"), QVariantList{}},
        {QStringLiteral("busy"), false},
        {QStringLiteral("activePack"), QString()},
        {QStringLiteral("progress"), 0.0},
        {QStringLiteral("catalogConfigured"), false},
    };
}

void configureEngine(QQmlEngine& engine, SettingsController& settings, bool darkMode = true) {
    engine.rootContext()->setContextProperty(QStringLiteral("config"), qmlConfig(darkMode));
    engine.rootContext()->setContextProperty(QStringLiteral("languagePacks"), qmlLanguagePacks());
    engine.rootContext()->setContextProperty(QStringLiteral("settings"), &settings);
    engine.rootContext()->setContextProperty(QStringLiteral("appVersion"), QStringLiteral("0.1.0"));
}

void configureView(QQuickView& view, SettingsController& settings, bool darkMode = true) {
    configureEngine(*view.engine(), settings, darkMode);
    view.setResizeMode(QQuickView::SizeViewToRootObject);
}

} // namespace

class QmlViewsTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void mainWindowLoadsHidden();
    void mainWindowUsesLightPaletteWhenConfigured();
    void lookupPopupLoads();
    void lookupPopupCloseButtonIsKeyboardFocusable();
    void lookupPopupShowsEntry();
    void lookupPopupShowsMessage();
    void lookupPopupRendersIpaAndTranslation();
    void settingsViewLoads();
    void settingsViewShowsLanguageSection();
    void settingsViewExposesAccessibility();
    void settingsViewSupportsKeyboardNavigation();
    void themeRespondsToDarkMode();
    void themeAppliesConfigOverrides();
};

void QmlViewsTest::mainWindowLoadsHidden() {
    SettingsController settings{Config{}};
    QQmlApplicationEngine engine;
    configureEngine(engine, settings);
    engine.rootContext()->setContextProperty(QStringLiteral("appVersion"), QStringLiteral("0.1.0"));

    engine.load(sourceUrl(QStringLiteral("ui/Main.qml")));

    QCOMPARE(engine.rootObjects().size(), 1);
    auto* window = qobject_cast<QQuickWindow*>(engine.rootObjects().constFirst());
    QVERIFY(window != nullptr);
    QVERIFY(!window->isVisible());
    QCOMPARE(window->title(), QStringLiteral("Lexicon Settings"));
}

void QmlViewsTest::mainWindowUsesLightPaletteWhenConfigured() {
    SettingsController settings{Config{}};
    QQmlApplicationEngine engine;
    configureEngine(engine, settings, false);
    engine.rootContext()->setContextProperty(QStringLiteral("appVersion"), QStringLiteral("0.1.0"));

    engine.load(sourceUrl(QStringLiteral("ui/Main.qml")));

    QCOMPARE(engine.rootObjects().size(), 1);
    auto* window = qobject_cast<QQuickWindow*>(engine.rootObjects().constFirst());
    QVERIFY(window != nullptr);
    QCOMPARE(window->color(), QColor(QStringLiteral("#f4f5f8")));
}

void QmlViewsTest::lookupPopupLoads() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);

    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.errors().isEmpty());
    QVERIFY(view.rootObject() != nullptr);
}

void QmlViewsTest::lookupPopupCloseButtonIsKeyboardFocusable() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);

    QVERIFY(view.rootObject()->property("focus").toBool());
    auto* closeButton = view.rootObject()->findChild<QQuickItem*>(QStringLiteral("closeButton"));
    QVERIFY(closeButton != nullptr);
    QVERIFY(closeButton->property("activeFocusOnTab").toBool());

    QAccessibleInterface* closeInterface = QAccessible::queryAccessibleInterface(closeButton);
    QVERIFY(closeInterface != nullptr);
    QCOMPARE(closeInterface->text(QAccessible::Name), QStringLiteral("Close"));
}

void QmlViewsTest::lookupPopupShowsEntry() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.rootObject() != nullptr);

    QVariantMap pronunciation;
    pronunciation.insert(QStringLiteral("respelling"), QStringLiteral("ser-uhn-DIP-i-tee"));
    pronunciation.insert(QStringLiteral("ipa"), QString());
    pronunciation.insert(QStringLiteral("text"), QStringLiteral("ser-uhn-DIP-i-tee"));

    QVariantMap definition;
    definition.insert(QStringLiteral("text"), QStringLiteral("A fortunate discovery by chance."));
    definition.insert(QStringLiteral("partOfSpeech"), QStringLiteral("noun"));
    definition.insert(QStringLiteral("label"), QString());

    QVariantMap entry;
    entry.insert(QStringLiteral("word"), QStringLiteral("serendipity"));
    entry.insert(QStringLiteral("pronunciation"), pronunciation);
    entry.insert(QStringLiteral("definitions"), QVariantList{definition});
    entry.insert(QStringLiteral("metadata"), QVariantMap{{QStringLiteral("source"), QStringLiteral("fixture")}});

    view.rootObject()->setProperty("loading", false);
    view.rootObject()->setProperty("hasMessage", false);
    view.rootObject()->setProperty("word", QStringLiteral("serendipity"));
    view.rootObject()->setProperty("entry", entry);
    QCoreApplication::processEvents();

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.errors().isEmpty());
    QCOMPARE(view.rootObject()->property("word").toString(), QStringLiteral("serendipity"));
    QVERIFY(view.rootObject()->property("entry").isValid());
    QVERIFY(view.height() > 0);
}

void QmlViewsTest::lookupPopupShowsMessage() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.rootObject() != nullptr);

    view.rootObject()->setProperty("loading", false);
    view.rootObject()->setProperty("hasMessage", true);
    view.rootObject()->setProperty("entry", QVariant());
    view.rootObject()->setProperty("messageTitle", QStringLiteral("No selection"));
    view.rootObject()->setProperty("message", QStringLiteral("Selection is unavailable."));
    QCoreApplication::processEvents();

    QCOMPARE(view.rootObject()->property("messageTitle").toString(), QStringLiteral("No selection"));
    QCOMPARE(view.rootObject()->property("message").toString(), QStringLiteral("Selection is unavailable."));
    QVERIFY(view.height() > 0);
}

void QmlViewsTest::lookupPopupRendersIpaAndTranslation() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/popup/LookupPopup.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);

    QVariantMap pronunciation{
        {QStringLiteral("ipa"), QStringLiteral("/ˌsɛrənˈdɪpɪti/")},
        {QStringLiteral("text"), QStringLiteral("/ˌsɛrənˈdɪpɪti/")},
        {QStringLiteral("style"), QStringLiteral("ipa")},
    };
    QVariantMap entry{
        {QStringLiteral("word"), QStringLiteral("serendipity")},
        {QStringLiteral("pronunciation"), pronunciation},
        {QStringLiteral("metadata"), QVariantMap{{QStringLiteral("translation"), QStringLiteral("Zufallstreffer")}}},
        {QStringLiteral("definitions"), QVariantList{}},
    };

    view.rootObject()->setProperty("loading", false);
    view.rootObject()->setProperty("hasMessage", false);
    view.rootObject()->setProperty("word", QStringLiteral("serendipity"));
    view.rootObject()->setProperty("entry", entry);
    QCoreApplication::processEvents();

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.errors().isEmpty());
    QVERIFY(view.height() > 0);
}

void QmlViewsTest::settingsViewShowsLanguageSection() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/settings/SettingsView.qml")));

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.errors().isEmpty());
    QVERIFY(view.rootObject() != nullptr);
}

void QmlViewsTest::settingsViewLoads() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/settings/SettingsView.qml")));

    QCOMPARE(view.status(), QQuickView::Ready);
    QVERIFY(view.errors().isEmpty());
    QVERIFY(view.rootObject() != nullptr);
    QVERIFY(view.rootObject()->property("darkMode").toBool());
}

void QmlViewsTest::settingsViewExposesAccessibility() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/settings/SettingsView.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);

    auto* root = view.rootObject();
    QVERIFY(root != nullptr);

    QAccessibleInterface* rootInterface = QAccessible::queryAccessibleInterface(root);
    QVERIFY(rootInterface != nullptr);
    QCOMPARE(rootInterface->role(), QAccessible::Pane);
    QCOMPARE(rootInterface->text(QAccessible::Name), QStringLiteral("Lexicon settings"));

    const QList<QPair<QString, QString>> controls{
        {QStringLiteral("dismissEscapeSwitch"), QStringLiteral("Dismiss on Escape")},
        {QStringLiteral("dismissOutsideSwitch"), QStringLiteral("Dismiss on outside click")},
        {QStringLiteral("themeCombo"), QStringLiteral("Theme")},
        {QStringLiteral("popupWidthSpin"), QStringLiteral("Popup width")},
        {QStringLiteral("animationCombo"), QStringLiteral("Animation")},
        {QStringLiteral("positionCombo"), QStringLiteral("Popup position")},
        {QStringLiteral("refreshCatalogButton"), QStringLiteral("Refresh language pack catalog")},
        {QStringLiteral("catalogUrlField"), QStringLiteral("Language pack catalog URL")},
        {QStringLiteral("showPronunciationSwitch"), QStringLiteral("Show pronunciation")},
        {QStringLiteral("pronunciationStyleCombo"), QStringLiteral("Pronunciation style")},
        {QStringLiteral("startOnLoginSwitch"), QStringLiteral("Start on login")},
        {QStringLiteral("cacheEnabledSwitch"), QStringLiteral("Lookup cache")},
        {QStringLiteral("cacheSizeSpin"), QStringLiteral("Cache size")},
        {QStringLiteral("saveButton"), QStringLiteral("Save settings")},
    };

    for (const auto& [objectName, accessibleName] : controls) {
        auto* item = root->findChild<QQuickItem*>(objectName);
        QVERIFY2(item != nullptr, qPrintable(objectName));

        QAccessibleInterface* interface = QAccessible::queryAccessibleInterface(item);
        QVERIFY2(interface != nullptr, qPrintable(objectName));
        QCOMPARE(interface->text(QAccessible::Name), accessibleName);
    }
}

void QmlViewsTest::settingsViewSupportsKeyboardNavigation() {
    SettingsController settings{Config{}};
    QQuickView view;
    configureView(view, settings);
    view.setSource(sourceUrl(QStringLiteral("ui/settings/SettingsView.qml")));
    QCOMPARE(view.status(), QQuickView::Ready);

    auto* root = view.rootObject();
    QVERIFY(root != nullptr);
    QVERIFY(root->isFocusScope());
    QVERIFY(root->property("focus").toBool());

    auto* first = root->findChild<QQuickItem*>(QStringLiteral("dismissEscapeSwitch"));
    auto* second = root->findChild<QQuickItem*>(QStringLiteral("dismissOutsideSwitch"));
    QVERIFY(first != nullptr);
    QVERIFY(second != nullptr);
    QVERIFY(first->property("activeFocusOnTab").toBool());
    QVERIFY(second->property("activeFocusOnTab").toBool());
    QCOMPARE(first->nextItemInFocusChain(), second);
    QCOMPARE(second->nextItemInFocusChain(false), first);
}

void QmlViewsTest::themeRespondsToDarkMode() {
    QQmlEngine engine;
    QQmlComponent component(&engine, sourceUrl(QStringLiteral("ui/themes/Theme.qml")));
    QVERIFY2(component.isReady(), qPrintable(component.errorString()));

    std::unique_ptr<QObject> theme(component.create());
    QVERIFY(theme != nullptr);

    theme->setProperty("darkMode", true);
    QCOMPARE(theme->property("background").value<QColor>(), QColor(QStringLiteral("#101218")));

    theme->setProperty("darkMode", false);
    QCOMPARE(theme->property("background").value<QColor>(), QColor(QStringLiteral("#f4f5f8")));
}

void QmlViewsTest::themeAppliesConfigOverrides() {
    QQmlEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("config"),
                                             QVariantMap{
                                                 {QStringLiteral("themeUsesDarkPalette"), true},
                                                 {QStringLiteral("cornerRadius"), 4},
                                                 {
                                                     QStringLiteral("themeColors"),
                                                     QVariantMap{
                                                         {QStringLiteral("background"), QStringLiteral("#112233")},
                                                         {QStringLiteral("accent"), QStringLiteral("#ff00aa")},
                                                     },
                                                 },
                                             });

    QQmlComponent component(&engine, sourceUrl(QStringLiteral("ui/themes/AppTheme.qml")));
    QVERIFY2(component.isReady(), qPrintable(component.errorString()));

    std::unique_ptr<QObject> theme(component.create());
    QVERIFY(theme != nullptr);

    QCOMPARE(theme->property("background").value<QColor>(), QColor(QStringLiteral("#112233")));
    QCOMPARE(theme->property("accent").value<QColor>(), QColor(QStringLiteral("#ff00aa")));
    QCOMPARE(theme->property("surface").value<QColor>(), QColor(QStringLiteral("#191c24")));
    QCOMPARE(theme->property("radius").toInt(), 4);
}

QTEST_MAIN(QmlViewsTest)

#include "test_qml_views.moc"

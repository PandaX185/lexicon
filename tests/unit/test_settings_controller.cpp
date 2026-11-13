#include "app/SettingsController.h"
#include "config/ConfigLoader.h"

#include <QFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace Lexicon;

namespace {

QString writeFile(const QString& path, const QString& contents) {
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        return {};
    }
    file.write(contents.toUtf8());
    file.close();
    return path;
}

QString readFile(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly)) {
        return {};
    }
    return QString::fromUtf8(file.readAll());
}

Config configAt(const QString& path) {
    Config config;
    config.configPath = path;
    return config;
}

} // namespace

class SettingsControllerTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void exposesConfiguration();
    void clampsPopupWidth();
    void clampsCacheSize();
    void ignoresUnsupportedEnumValues();
    void saveWritesAndReloadPicksUpValues();
    void saveEmitsConfigMap();
    void saveBacksUpCommentsAndUnknownKeys();
    void saveCreatesParentDirectory();
    void reloadReadsValuesFromDisk();
};

void SettingsControllerTest::exposesConfiguration() {
    Config config;
    config.general.startOnLogin = true;
    config.appearance.theme = Theme::Light;
    config.performance.cacheSize = 512;
    SettingsController settings(config);

    QCOMPARE(settings.startOnLogin(), true);
    QCOMPARE(settings.theme(), QStringLiteral("light"));
    QCOMPARE(settings.cacheSize(), 512);
    QCOMPARE(settings.lookup(), QStringLiteral("SUPER+SHIFT+D"));
}

void SettingsControllerTest::clampsPopupWidth() {
    SettingsController settings(Config{});
    QSignalSpy spy(&settings, &SettingsController::popupWidthChanged);

    settings.setPopupWidth(10);
    QCOMPARE(settings.popupWidth(), 240);
    settings.setPopupWidth(5000);
    QCOMPARE(settings.popupWidth(), 1200);

    QCOMPARE(spy.count(), 2);
}

void SettingsControllerTest::clampsCacheSize() {
    SettingsController settings(Config{});

    settings.setCacheSize(-10);
    QCOMPARE(settings.cacheSize(), 0);
    settings.setCacheSize(99999);
    QCOMPARE(settings.cacheSize(), 4096);
}

void SettingsControllerTest::ignoresUnsupportedEnumValues() {
    SettingsController settings(Config{});

    settings.setTheme(QStringLiteral("solarized"));
    settings.setAnimation(QStringLiteral("bouncy"));
    settings.setPosition(QStringLiteral("top"));
    settings.setPronunciationStyle(QStringLiteral("sampa"));

    QCOMPARE(settings.theme(), QStringLiteral("dark"));
    QCOMPARE(settings.animation(), QStringLiteral("subtle"));
    QCOMPARE(settings.position(), QStringLiteral("near_selection"));
    QCOMPARE(settings.pronunciationStyle(), QStringLiteral("respelling"));
}

void SettingsControllerTest::saveWritesAndReloadPicksUpValues() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));

    SettingsController settings(configAt(path));
    settings.setTheme(QStringLiteral("light"));
    settings.setPopupWidth(500);
    settings.setCacheSize(64);
    settings.setStartOnLogin(true);
    settings.setSourceLanguage(QStringLiteral("en"));
    settings.setTargetLanguage(QStringLiteral("de"));

    QVERIFY(settings.save());

    const ConfigLoadResult loaded = ConfigLoader{}.load(path);
    QVERIFY(loaded.found);
    QCOMPARE(loaded.config.appearance.theme, Theme::Light);
    QCOMPARE(loaded.config.appearance.popupWidth, 500);
    QCOMPARE(loaded.config.performance.cacheSize, 64);
    QCOMPARE(loaded.config.general.startOnLogin, true);
    QCOMPARE(loaded.config.dictionary.targetLanguage, QStringLiteral("de"));

    // Mutate the in-memory copy, then reload to prove disk is the source.
    settings.setTheme(QStringLiteral("dark"));
    settings.setPopupWidth(400);
    settings.reload();

    QCOMPARE(settings.theme(), QStringLiteral("light"));
    QCOMPARE(settings.popupWidth(), 500);
}

void SettingsControllerTest::saveEmitsConfigMap() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));

    SettingsController settings(configAt(path));
    settings.setTheme(QStringLiteral("light"));
    QSignalSpy spy(&settings, &SettingsController::configSaved);

    QVERIFY(settings.save());

    QCOMPARE(spy.count(), 1);
    const QVariantMap map = spy.constFirst().constFirst().toMap();
    QCOMPARE(map.value(QStringLiteral("theme")).toString(), QStringLiteral("light"));
}

void SettingsControllerTest::saveBacksUpCommentsAndUnknownKeys() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));
    const QString original = QStringLiteral(R"(
# hand-written comment
[appearance]
theme = "light"
my_custom_key = 42
)");
    QVERIFY(!writeFile(path, original).isEmpty());

    SettingsController settings(configAt(path));
    settings.setPopupWidth(480);
    QVERIFY(settings.save());

    const QString backup = path + QStringLiteral(".bak");
    QVERIFY(QFile::exists(backup));
    QCOMPARE(readFile(backup), original);

    const QString saved = readFile(path);
    QVERIFY(saved.contains(QStringLiteral("popup_width = 480")));
    QVERIFY(!saved.contains(QStringLiteral("my_custom_key")));
}

void SettingsControllerTest::saveCreatesParentDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("nested/config/config.toml"));

    SettingsController settings(configAt(path));
    settings.setTheme(QStringLiteral("system"));
    QVERIFY(settings.save());
    QVERIFY(QFile::exists(path));
}

void SettingsControllerTest::reloadReadsValuesFromDisk() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));
    QVERIFY(!writeFile(path, QStringLiteral(R"(
[appearance]
theme = "system"
popup_width = 620

[performance]
cache_size = 32
)"))
                 .isEmpty());

    SettingsController settings(configAt(path));
    settings.reload();

    QCOMPARE(settings.theme(), QStringLiteral("system"));
    QCOMPARE(settings.popupWidth(), 620);
    QCOMPARE(settings.cacheSize(), 32);
}

QTEST_MAIN(SettingsControllerTest)

#include "test_settings_controller.moc"

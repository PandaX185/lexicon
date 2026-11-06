#include "config/ConfigLoader.h"

#include <QFile>
#include <QTemporaryDir>
#include <QTest>

#include <algorithm>

using namespace Lexicon;

class ConfigLoaderTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void defaultsWhenMissing();
    void loadsValues();
    void reportsUnknownKeys();
    void clampsPopupWidth();
    void resolvesRelativeDatabasePath();
    void rejectsInvalidToml();
    void normalizesSuperShortcut();

  private:
    static QString writeConfig(const QTemporaryDir& directory, const QString& contents) {
        QString path = directory.filePath(QStringLiteral("config.toml"));
        QFile file(path);
        if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
            return {};
        }
        file.write(contents.toUtf8());
        file.close();
        return path;
    }

    static bool hasIssue(const ConfigLoadResult& result, const QString& key) {
        return std::ranges::any_of(result.issues, [&key](const ConfigIssue& issue) { return issue.key == key; });
    }
};

void ConfigLoaderTest::defaultsWhenMissing() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(directory.filePath(QStringLiteral("missing.toml")));

    QVERIFY(!result.found);
    QVERIFY(result.usedDefaults);
    QCOMPARE(result.config.appearance.popupWidth, 360);
    QCOMPARE(result.config.hotkeys.lookup, QStringLiteral("SUPER+SHIFT+D"));
    QCOMPARE(result.config.general.offlineFirst, true);
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Respelling);
}

void ConfigLoaderTest::loadsValues() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[general]
start_on_login = true
offline_first = false

[interaction]
activation = "global_shortcut"
clipboard_fallback = true
dismiss_on_escape = false
dismiss_on_outside_click = false

[hotkeys]
lookup = "CTRL+ALT+L"

[appearance]
theme = "light"
popup_width = 480
animation = "none"
position = "center"

[dictionary]
database_path = "relative.sqlite"
show_pronunciation = false
pronunciation_style = "respelling"

[performance]
cache_enabled = false
lazy_initialize = false
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(result.found);
    QVERIFY(!result.usedDefaults);
    QVERIFY(result.issues.isEmpty());
    QCOMPARE(result.config.general.startOnLogin, true);
    QCOMPARE(result.config.general.offlineFirst, false);
    QCOMPARE(result.config.interaction.clipboardFallback, true);
    QCOMPARE(result.config.interaction.dismissOnEscape, false);
    QCOMPARE(result.config.appearance.theme, Theme::Light);
    QCOMPARE(result.config.appearance.popupWidth, 480);
    QCOMPARE(result.config.appearance.animation, Animation::None);
    QCOMPARE(result.config.appearance.position, PopupPosition::Center);
    QCOMPARE(result.config.dictionary.showPronunciation, false);
    QCOMPARE(result.config.performance.cacheEnabled, false);
    QCOMPARE(result.config.performance.lazyInitialize, false);
    QCOMPARE(result.config.lookupKeySequence().toString(QKeySequence::PortableText), QStringLiteral("Ctrl+Alt+L"));
    QCOMPARE(result.config.dictionary.databasePath, directory.filePath(QStringLiteral("relative.sqlite")));
}

void ConfigLoaderTest::reportsUnknownKeys() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[appearance]
popup_width = 400
mystery_option = true
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(result.found);
    QVERIFY(hasIssue(result, QStringLiteral("appearance.mystery_option")));
}

void ConfigLoaderTest::clampsPopupWidth() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[appearance]
popup_width = 9999
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.appearance.popupWidth, 1200);
    QVERIFY(hasIssue(result, QStringLiteral("appearance.popup_width")));
}

void ConfigLoaderTest::resolvesRelativeDatabasePath() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[dictionary]
database_path = "data/en.sqlite"
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.dictionary.databasePath, directory.filePath(QStringLiteral("data/en.sqlite")));
}

void ConfigLoaderTest::rejectsInvalidToml() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral("[appearance\npopup_width = 1\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(result.found);
    QVERIFY(result.usedDefaults);
    bool hasError = false;
    for (const ConfigIssue& issue : result.issues) {
        if (issue.isError()) {
            hasError = true;
            break;
        }
    }
    QVERIFY(hasError);
    QCOMPARE(result.config.appearance.popupWidth, 360);
}

void ConfigLoaderTest::normalizesSuperShortcut() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[hotkeys]
lookup = "SUPER+SHIFT+D"
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.lookupKeySequence().toString(QKeySequence::PortableText), QStringLiteral("Meta+Shift+D"));
}

QTEST_GUILESS_MAIN(ConfigLoaderTest)

#include "test_config.moc"

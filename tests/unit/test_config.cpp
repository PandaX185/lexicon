#include "config/ConfigLoader.h"
#include "support/ScopedEnvironment.h"

#include <QDir>
#include <QFile>
#include <QStandardPaths>
#include <QTemporaryDir>
#include <QTest>

#include <algorithm>

using namespace Lexicon;
using Lexicon::Test::ScopedEnvironment;

class ConfigLoaderTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void defaultsWhenMissing();
    void loadsValues();
    void reportsUnknownKeys();
    void reportsTopLevelUnknownKey();
    void clampsPopupWidth();
    void clampsPopupWidthAtBothBounds();
    void resolvesRelativeDatabasePath();
    void preservesAbsoluteDatabasePath();
    void resolvesUnicodeRelativeDatabasePath();
    void rejectsInvalidToml();
    void rejectsDuplicateKeys();
    void rejectsDirectoryPath();
    void normalizesSuperShortcut();
    void keepsDefaultForEmptyShortcut();
    void keepsDefaultForInvalidShortcut();
    void parsesKeySequence_data();
    void parsesKeySequence();
    void keepsDefaultsForWrongTypes();
    void keepsDefaultsForNonTableSection();
    void keepsDefaultsForUnsupportedEnumValues();
    void loadsIpaPronunciationStyle();
    void explicitPathOverridesEnvironment();
    void environmentConfigPathIsUsed();
    void defaultPathsHaveExpectedSuffixes();
    void toVariantMapExposesResolvedValues();
    void systemThemeFollowsPlatformColorScheme();
    void saveRoundTripsValues();
    void saveUsesConfigPath();
    void saveCreatesParentDirectory();
    void saveEscapesStringValues();
    void saveBacksUpUnknownContent();

  private:
    static QString writeFile(const QString& path, const QString& contents) {
        QFile file(path);
        if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
            return {};
        }
        file.write(contents.toUtf8());
        file.close();
        return path;
    }

    static QString writeConfig(const QTemporaryDir& directory, const QString& contents,
                               const QString& name = QStringLiteral("config.toml")) {
        return writeFile(directory.filePath(name), contents);
    }

    static bool hasIssue(const ConfigLoadResult& result, const QString& key) {
        return std::ranges::any_of(result.issues, [&key](const ConfigIssue& issue) { return issue.key == key; });
    }

    static bool hasError(const ConfigLoadResult& result) {
        return std::ranges::any_of(result.issues, [](const ConfigIssue& issue) { return issue.isError(); });
    }
};

void ConfigLoaderTest::defaultsWhenMissing() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(directory.filePath(QStringLiteral("missing.toml")));

    QVERIFY(!result.found);
    QVERIFY(result.usedDefaults);
    QVERIFY(result.issues.isEmpty());
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
source_language = "EN_GB"
target_language = "de"
show_pronunciation = false
pronunciation_style = "ipa"

[translation]
catalog_url = "  https://example.org/catalog.json  "

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
    QCOMPARE(result.config.dictionary.sourceLanguage, QStringLiteral("en-gb"));
    QCOMPARE(result.config.dictionary.targetLanguage, QStringLiteral("de"));
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Ipa);
    QCOMPARE(result.config.translation.catalogUrl, QStringLiteral("https://example.org/catalog.json"));
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

void ConfigLoaderTest::reportsTopLevelUnknownKey() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral("mystery = true\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(hasIssue(result, QStringLiteral("mystery")));
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

void ConfigLoaderTest::clampsPopupWidthAtBothBounds() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const QString lowPath =
        writeConfig(directory, QStringLiteral("[appearance]\npopup_width = 1\n"), QStringLiteral("low.toml"));
    const QString highPath =
        writeConfig(directory, QStringLiteral("[appearance]\npopup_width = 240\n"), QStringLiteral("boundary.toml"));
    QVERIFY(!lowPath.isEmpty());
    QVERIFY(!highPath.isEmpty());

    ConfigLoader loader;
    QCOMPARE(loader.load(lowPath).config.appearance.popupWidth, 240);
    QCOMPARE(loader.load(highPath).config.appearance.popupWidth, 240);
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

void ConfigLoaderTest::preservesAbsoluteDatabasePath() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString absolutePath = directory.filePath(QStringLiteral("absolute.sqlite"));
    const QString path =
        writeConfig(directory, QStringLiteral("[dictionary]\ndatabase_path = \"%1\"\n").arg(absolutePath));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.dictionary.databasePath, absolutePath);
}

void ConfigLoaderTest::resolvesUnicodeRelativeDatabasePath() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path =
        writeConfig(directory, QStringLiteral("[dictionary]\ndatabase_path = \"données/en.sqlite\"\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.dictionary.databasePath, directory.filePath(QStringLiteral("données/en.sqlite")));
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
    QVERIFY(hasError(result));
    QCOMPARE(result.config.appearance.popupWidth, 360);
}

void ConfigLoaderTest::rejectsDuplicateKeys() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[appearance]
popup_width = 300
popup_width = 400
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(hasError(result));
    QCOMPARE(result.config.appearance.popupWidth, 360);
}

void ConfigLoaderTest::rejectsDirectoryPath() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(directory.path());

    QVERIFY(result.found);
    QVERIFY(result.usedDefaults);
    QVERIFY(hasError(result));
    QVERIFY(std::ranges::any_of(result.issues, [](const ConfigIssue& issue) {
        return issue.isError() && issue.key == QStringLiteral("<file>") &&
               issue.message.contains(QStringLiteral("not a regular file"));
    }));
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

void ConfigLoaderTest::keepsDefaultForEmptyShortcut() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral("[hotkeys]\nlookup = \"\"\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.hotkeys.lookup, QStringLiteral("SUPER+SHIFT+D"));
    QCOMPARE(result.config.lookupKeySequence().toString(QKeySequence::PortableText), QStringLiteral("Meta+Shift+D"));
}

void ConfigLoaderTest::keepsDefaultForInvalidShortcut() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral("[hotkeys]\nlookup = \"   \"\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.hotkeys.lookup, QStringLiteral("SUPER+SHIFT+D"));
    QVERIFY(hasIssue(result, QStringLiteral("hotkeys.lookup")));
}

void ConfigLoaderTest::parsesKeySequence_data() {
    QTest::addColumn<QString>("input");
    QTest::addColumn<QString>("expected");

    QTest::newRow("empty") << QString() << QString();
    QTest::newRow("whitespace") << QStringLiteral("   ") << QString();
    QTest::newRow("super-upper") << QStringLiteral("SUPER+SHIFT+D") << QStringLiteral("Meta+Shift+D");
    QTest::newRow("super-lower") << QStringLiteral("super+shift+d") << QStringLiteral("Meta+Shift+D");
    QTest::newRow("ctrl-alt") << QStringLiteral("CTRL+ALT+L") << QStringLiteral("Ctrl+Alt+L");
    QTest::newRow("meta") << QStringLiteral("Meta+Shift+D") << QStringLiteral("Meta+Shift+D");
    QTest::newRow("trimmed") << QStringLiteral("  Ctrl+Space  ") << QStringLiteral("Ctrl+Space");
}

void ConfigLoaderTest::parsesKeySequence() {
    QFETCH(QString, input);
    QFETCH(QString, expected);

    QCOMPARE(parseKeySequence(input).toString(QKeySequence::PortableText), expected);
}

void ConfigLoaderTest::keepsDefaultsForWrongTypes() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[general]
start_on_login = "yes"

[interaction]
dismiss_on_escape = 1

[hotkeys]
lookup = 5

[appearance]
theme = 42
popup_width = "wide"
animation = true
position = []

[dictionary]
database_path = false
show_pronunciation = "true"

[performance]
cache_enabled = 1
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.general.startOnLogin, false);
    QCOMPARE(result.config.interaction.dismissOnEscape, true);
    QCOMPARE(result.config.hotkeys.lookup, QStringLiteral("SUPER+SHIFT+D"));
    QCOMPARE(result.config.appearance.theme, Theme::Dark);
    QCOMPARE(result.config.appearance.popupWidth, 360);
    QCOMPARE(result.config.appearance.animation, Animation::Subtle);
    QCOMPARE(result.config.appearance.position, PopupPosition::NearSelection);
    QCOMPARE(result.config.dictionary.databasePath, QString());
    QCOMPARE(result.config.dictionary.showPronunciation, true);
    QCOMPARE(result.config.performance.cacheEnabled, true);

    QVERIFY(hasIssue(result, QStringLiteral("general.start_on_login")));
    QVERIFY(hasIssue(result, QStringLiteral("interaction.dismiss_on_escape")));
    QVERIFY(hasIssue(result, QStringLiteral("hotkeys.lookup")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.theme")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.popup_width")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.animation")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.position")));
    QVERIFY(hasIssue(result, QStringLiteral("dictionary.database_path")));
    QVERIFY(hasIssue(result, QStringLiteral("dictionary.show_pronunciation")));
    QVERIFY(hasIssue(result, QStringLiteral("performance.cache_enabled")));
}

void ConfigLoaderTest::keepsDefaultsForNonTableSection() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral("interaction = \"not a table\"\n"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(hasIssue(result, QStringLiteral("interaction")));
    QCOMPARE(result.config.interaction.dismissOnEscape, true);
}

void ConfigLoaderTest::keepsDefaultsForUnsupportedEnumValues() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[interaction]
activation = "clipboard"

[appearance]
theme = "solarized"
animation = "bouncy"
position = "top"

[dictionary]
pronunciation_style = "sampa"
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QCOMPARE(result.config.interaction.activation, Activation::GlobalShortcut);
    QCOMPARE(result.config.appearance.theme, Theme::Dark);
    QCOMPARE(result.config.appearance.animation, Animation::Subtle);
    QCOMPARE(result.config.appearance.position, PopupPosition::NearSelection);
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Respelling);
    QVERIFY(hasIssue(result, QStringLiteral("interaction.activation")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.theme")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.animation")));
    QVERIFY(hasIssue(result, QStringLiteral("appearance.position")));
    QVERIFY(hasIssue(result, QStringLiteral("dictionary.pronunciation_style")));
}

void ConfigLoaderTest::loadsIpaPronunciationStyle() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = writeConfig(directory, QStringLiteral(R"(
[dictionary]
show_pronunciation = true
pronunciation_style = "ipa"
)"));
    QVERIFY(!path.isEmpty());

    ConfigLoader loader;
    const ConfigLoadResult result = loader.load(path);

    QVERIFY(result.found);
    QVERIFY(!result.usedDefaults);
    QVERIFY(result.issues.isEmpty());
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Ipa);
    QCOMPARE(pronunciationStyleToString(result.config.dictionary.pronunciationStyle), QStringLiteral("ipa"));
}

void ConfigLoaderTest::explicitPathOverridesEnvironment() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString environmentPath =
        writeConfig(directory, QStringLiteral("[appearance]\npopup_width = 400\n"), QStringLiteral("env.toml"));
    const QString explicitPath =
        writeConfig(directory, QStringLiteral("[appearance]\npopup_width = 500\n"), QStringLiteral("explicit.toml"));
    QVERIFY(!environmentPath.isEmpty());
    QVERIFY(!explicitPath.isEmpty());

    const ScopedEnvironment environment("LEXICON_CONFIG", environmentPath.toUtf8());
    ConfigLoader loader;

    QCOMPARE(loader.load(explicitPath).config.appearance.popupWidth, 500);
    QCOMPARE(loader.load().config.appearance.popupWidth, 400);
}

void ConfigLoaderTest::environmentConfigPathIsUsed() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString environmentPath =
        writeConfig(directory, QStringLiteral("[appearance]\npopup_width = 410\n"), QStringLiteral("env.toml"));
    QVERIFY(!environmentPath.isEmpty());

    const ScopedEnvironment environment("LEXICON_CONFIG", environmentPath.toUtf8());
    ConfigLoader loader;
    const ConfigLoadResult result = loader.load();

    QVERIFY(result.found);
    QCOMPARE(result.config.appearance.popupWidth, 410);
    QCOMPARE(result.sourcePath, environmentPath);
}

void ConfigLoaderTest::defaultPathsHaveExpectedSuffixes() {
    const QString configPath = ConfigLoader::defaultConfigPath();
    const QString dataDir = ConfigLoader::defaultDataDir();
    const QString databasePath = ConfigLoader::defaultDatabasePath();

    QVERIFY(configPath.endsWith(QStringLiteral("/lexicon/config.toml")));
    QVERIFY(dataDir.endsWith(QStringLiteral("/lexicon")));
    QVERIFY(databasePath.endsWith(QStringLiteral("/lexicon/dictionaries/en.sqlite")));
}

void ConfigLoaderTest::toVariantMapExposesResolvedValues() {
    Config config;
    config.general.startOnLogin = true;
    config.interaction.dismissOnEscape = false;
    config.hotkeys.lookup = QStringLiteral("CTRL+ALT+L");
    config.appearance.theme = Theme::System;
    config.appearance.popupWidth = 500;
    config.dictionary.databasePath = QStringLiteral("/tmp/en.sqlite");

    const QVariantMap lightMap = config.toVariantMap(Qt::ColorScheme::Light);
    QCOMPARE(lightMap.value(QStringLiteral("startOnLogin")).toBool(), true);
    QCOMPARE(lightMap.value(QStringLiteral("dismissOnEscape")).toBool(), false);
    QCOMPARE(lightMap.value(QStringLiteral("lookup")).toString(), QStringLiteral("CTRL+ALT+L"));
    QCOMPARE(lightMap.value(QStringLiteral("theme")).toString(), QStringLiteral("system"));
    QCOMPARE(lightMap.value(QStringLiteral("themeUsesDarkPalette")).toBool(), false);
    QCOMPARE(lightMap.value(QStringLiteral("popupWidth")).toInt(), 500);
    QCOMPARE(lightMap.value(QStringLiteral("databasePath")).toString(), QStringLiteral("/tmp/en.sqlite"));

    const QVariantMap darkMap = config.toVariantMap(Qt::ColorScheme::Dark);
    QCOMPARE(darkMap.value(QStringLiteral("themeUsesDarkPalette")).toBool(), true);
}

void ConfigLoaderTest::systemThemeFollowsPlatformColorScheme() {
    QVERIFY(resolveDarkPalette(Theme::Dark, Qt::ColorScheme::Light));
    QVERIFY(!resolveDarkPalette(Theme::Light, Qt::ColorScheme::Dark));
    QVERIFY(!resolveDarkPalette(Theme::System, Qt::ColorScheme::Light));
    QVERIFY(resolveDarkPalette(Theme::System, Qt::ColorScheme::Dark));
    QVERIFY(resolveDarkPalette(Theme::System, Qt::ColorScheme::Unknown));
}

void ConfigLoaderTest::saveRoundTripsValues() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));

    Config config;
    config.general.startOnLogin = true;
    config.general.offlineFirst = false;
    config.interaction.dismissOnEscape = false;
    config.hotkeys.lookup = QStringLiteral("CTRL+ALT+L");
    config.appearance.theme = Theme::Light;
    config.appearance.popupWidth = 520;
    config.appearance.animation = Animation::None;
    config.appearance.position = PopupPosition::Cursor;
    config.dictionary.directory = QStringLiteral("/tmp/dictionaries");
    config.dictionary.sourceLanguage = QStringLiteral("en");
    config.dictionary.targetLanguage = QStringLiteral("de");
    config.dictionary.showPronunciation = false;
    config.dictionary.pronunciationStyle = PronunciationStyle::Ipa;
    config.translation.catalogUrl = QStringLiteral("https://example.org/catalog.json");
    config.performance.cacheEnabled = false;
    config.performance.cacheSize = 16;
    config.performance.lazyInitialize = false;

    ConfigLoader loader;
    QVERIFY(loader.save(config, path));

    const ConfigLoadResult result = loader.load(path);
    QVERIFY(result.found);
    QVERIFY(result.issues.isEmpty());
    QCOMPARE(result.config.general.startOnLogin, true);
    QCOMPARE(result.config.general.offlineFirst, false);
    QCOMPARE(result.config.interaction.dismissOnEscape, false);
    QCOMPARE(result.config.hotkeys.lookup, QStringLiteral("CTRL+ALT+L"));
    QCOMPARE(result.config.appearance.theme, Theme::Light);
    QCOMPARE(result.config.appearance.popupWidth, 520);
    QCOMPARE(result.config.appearance.animation, Animation::None);
    QCOMPARE(result.config.appearance.position, PopupPosition::Cursor);
    QCOMPARE(result.config.dictionary.directory, QStringLiteral("/tmp/dictionaries"));
    QCOMPARE(result.config.dictionary.targetLanguage, QStringLiteral("de"));
    QCOMPARE(result.config.dictionary.showPronunciation, false);
    QCOMPARE(result.config.dictionary.pronunciationStyle, PronunciationStyle::Ipa);
    QCOMPARE(result.config.translation.catalogUrl, QStringLiteral("https://example.org/catalog.json"));
    QCOMPARE(result.config.performance.cacheEnabled, false);
    QCOMPARE(result.config.performance.cacheSize, 16);
    QCOMPARE(result.config.performance.lazyInitialize, false);
}

void ConfigLoaderTest::saveUsesConfigPath() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));

    Config config = Config{};
    config.configPath = path;
    config.appearance.popupWidth = 440;

    ConfigLoader loader;
    QVERIFY(loader.save(config, QString()));
    QCOMPARE(loader.load(path).config.appearance.popupWidth, 440);
}

void ConfigLoaderTest::saveCreatesParentDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("nested/dir/config.toml"));

    ConfigLoader loader;
    QVERIFY(loader.save(Config{}, path));
    QVERIFY(QFile::exists(path));
}

void ConfigLoaderTest::saveEscapesStringValues() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));
    const QString tricky = QStringLiteral("https://example.org/a\"b\\c");

    Config config = Config{};
    config.translation.catalogUrl = tricky;

    ConfigLoader loader;
    QVERIFY(loader.save(config, path));
    QCOMPARE(loader.load(path).config.translation.catalogUrl, tricky);
}

void ConfigLoaderTest::saveBacksUpUnknownContent() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("config.toml"));
    const QString original = QStringLiteral("# keep me\n[appearance]\npopup_width = 300\ncustom_flag = true\n");
    QCOMPARE(writeConfig(directory, original), path);

    ConfigLoader loader;
    QVERIFY(loader.save(Config{}, path));

    const QString backup = path + QStringLiteral(".bak");
    QVERIFY(QFile::exists(backup));
    QFile file(backup);
    QVERIFY(file.open(QIODevice::ReadOnly));
    QCOMPARE(QString::fromUtf8(file.readAll()), original);
}

QTEST_GUILESS_MAIN(ConfigLoaderTest)

#include "test_config.moc"

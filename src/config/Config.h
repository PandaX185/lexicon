#pragma once

#include <QKeySequence>
#include <QString>
#include <QVariantMap>
#include <Qt>

namespace Lexicon {

enum class Activation {
    GlobalShortcut,
};

enum class Theme {
    Dark,
    Light,
    System,
};

enum class Animation {
    None,
    Subtle,
};

enum class PopupPosition {
    NearSelection,
    Center,
    Cursor,
};

enum class PronunciationStyle {
    Respelling,
    Ipa,
};

struct GeneralConfig {
    bool startOnLogin = false;
    bool offlineFirst = true;
};

struct InteractionConfig {
    Activation activation = Activation::GlobalShortcut;
    bool clipboardFallback = false;
    bool dismissOnEscape = true;
    bool dismissOnOutsideClick = true;
};

struct HotkeysConfig {
    QString lookup = QStringLiteral("SUPER+SHIFT+D");
};

struct AppearanceConfig {
    Theme theme = Theme::Dark;
    int popupWidth = 360;
    Animation animation = Animation::Subtle;
    PopupPosition position = PopupPosition::NearSelection;
};

struct DictionaryConfig {
    // An explicit single-file override. When empty, the dictionaries in
    // `directory` are searched for the configured language pair.
    QString databasePath;
    QString directory;
    QString sourceLanguage = QStringLiteral("en");
    QString targetLanguage = QStringLiteral("en");
    bool showPronunciation = true;
    PronunciationStyle pronunciationStyle = PronunciationStyle::Respelling;
};

// Translation is optional: with no catalog configured the settings window can
// still list what is installed, but nothing can be downloaded.
struct TranslationConfig {
    QString catalogUrl;
};

struct PerformanceConfig {
    bool cacheEnabled = true;
    int cacheSize = 256;
    bool lazyInitialize = true;
};

struct Config {
    GeneralConfig general;
    InteractionConfig interaction;
    HotkeysConfig hotkeys;
    AppearanceConfig appearance;
    DictionaryConfig dictionary;
    TranslationConfig translation;
    PerformanceConfig performance;

    // Where this configuration was loaded from (or where it should be saved).
    QString configPath;

    [[nodiscard]] QString resolvedDatabasePath() const;
    [[nodiscard]] QString resolvedDictionaryDirectory() const;
    [[nodiscard]] QStringList resolvedDictionaryDirectories() const;
    [[nodiscard]] bool usesDictionaryDirectory() const;
    [[nodiscard]] QKeySequence lookupKeySequence() const;
    [[nodiscard]] QVariantMap toVariantMap(Qt::ColorScheme systemScheme = Qt::ColorScheme::Unknown) const;
};

// Resolves the configured theme against the platform color scheme. `system` follows
// the platform scheme; when the platform does not report one, it falls back to dark.
[[nodiscard]] bool resolveDarkPalette(Theme theme, Qt::ColorScheme systemScheme);

[[nodiscard]] QString activationToString(Activation value);
[[nodiscard]] QString themeToString(Theme value);
[[nodiscard]] QString animationToString(Animation value);
[[nodiscard]] QString popupPositionToString(PopupPosition value);
[[nodiscard]] QString pronunciationStyleToString(PronunciationStyle value);

[[nodiscard]] QKeySequence parseKeySequence(const QString& value);

} // namespace Lexicon

#pragma once

#include <QKeySequence>
#include <QString>
#include <QVariantMap>

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
    QString databasePath;
    bool showPronunciation = true;
    PronunciationStyle pronunciationStyle = PronunciationStyle::Respelling;
};

struct PerformanceConfig {
    bool cacheEnabled = true;
    bool lazyInitialize = true;
};

struct Config {
    GeneralConfig general;
    InteractionConfig interaction;
    HotkeysConfig hotkeys;
    AppearanceConfig appearance;
    DictionaryConfig dictionary;
    PerformanceConfig performance;

    [[nodiscard]] QString resolvedDatabasePath() const;
    [[nodiscard]] QKeySequence lookupKeySequence() const;
    [[nodiscard]] QVariantMap toVariantMap() const;

    // Convenience for QML: explicit boolean that matches the current palette intent.
    // `system` currently behaves like `dark`, so this is intentionally not a separate
    // theme path.
    [[nodiscard]] bool appearanceUsesDarkPalette() const;
};

[[nodiscard]] QString activationToString(Activation value);
[[nodiscard]] QString themeToString(Theme value);
[[nodiscard]] QString animationToString(Animation value);
[[nodiscard]] QString popupPositionToString(PopupPosition value);
[[nodiscard]] QString pronunciationStyleToString(PronunciationStyle value);

[[nodiscard]] QKeySequence parseKeySequence(const QString& value);

} // namespace Lexicon

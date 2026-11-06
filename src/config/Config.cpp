#include "config/Config.h"

#include <QStandardPaths>

namespace Lexicon {

QString activationToString(Activation value) {
    switch (value) {
    case Activation::GlobalShortcut:
        return QStringLiteral("global_shortcut");
    }
    return QStringLiteral("global_shortcut");
}

QString themeToString(Theme value) {
    switch (value) {
    case Theme::Dark:
        return QStringLiteral("dark");
    case Theme::Light:
        return QStringLiteral("light");
    case Theme::System:
        return QStringLiteral("system");
    }
    return QStringLiteral("dark");
}

QString animationToString(Animation value) {
    switch (value) {
    case Animation::None:
        return QStringLiteral("none");
    case Animation::Subtle:
        return QStringLiteral("subtle");
    }
    return QStringLiteral("subtle");
}

QString popupPositionToString(PopupPosition value) {
    switch (value) {
    case PopupPosition::NearSelection:
        return QStringLiteral("near_selection");
    case PopupPosition::Center:
        return QStringLiteral("center");
    case PopupPosition::Cursor:
        return QStringLiteral("cursor");
    }
    return QStringLiteral("near_selection");
}

QString pronunciationStyleToString(PronunciationStyle value) {
    switch (value) {
    case PronunciationStyle::Respelling:
        return QStringLiteral("respelling");
    }
    return QStringLiteral("respelling");
}

QKeySequence parseKeySequence(const QString& value) {
    QString normalized = value.trimmed();
    if (normalized.isEmpty()) {
        return {};
    }
    normalized.replace(QStringLiteral("SUPER"), QStringLiteral("Meta"), Qt::CaseInsensitive);
    return QKeySequence::fromString(normalized, QKeySequence::PortableText);
}

QString Config::resolvedDatabasePath() const {
    if (!dictionary.databasePath.trimmed().isEmpty()) {
        return dictionary.databasePath;
    }
    return QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation) +
           QStringLiteral("/lexicon/dictionaries/en.sqlite");
}

QKeySequence Config::lookupKeySequence() const {
    return parseKeySequence(hotkeys.lookup);
}

bool Config::appearanceUsesDarkPalette() const {
    // `system` is a supported TOML value, but the palette does not yet probe the
    // desktop theme. For now `system` behaves like `dark`.
    return appearance.theme != Theme::Light;
}

QVariantMap Config::toVariantMap() const {
    QVariantMap map;
    map.insert(QStringLiteral("startOnLogin"), general.startOnLogin);
    map.insert(QStringLiteral("offlineFirst"), general.offlineFirst);

    map.insert(QStringLiteral("activation"), activationToString(interaction.activation));
    map.insert(QStringLiteral("clipboardFallback"), interaction.clipboardFallback);
    map.insert(QStringLiteral("dismissOnEscape"), interaction.dismissOnEscape);
    map.insert(QStringLiteral("dismissOnOutsideClick"), interaction.dismissOnOutsideClick);

    map.insert(QStringLiteral("lookup"), hotkeys.lookup);

    map.insert(QStringLiteral("theme"), themeToString(appearance.theme));
    map.insert(QStringLiteral("themeUsesDarkPalette"), appearanceUsesDarkPalette());
    map.insert(QStringLiteral("popupWidth"), appearance.popupWidth);
    map.insert(QStringLiteral("animation"), animationToString(appearance.animation));
    map.insert(QStringLiteral("position"), popupPositionToString(appearance.position));

    map.insert(QStringLiteral("databasePath"), resolvedDatabasePath());
    map.insert(QStringLiteral("showPronunciation"), dictionary.showPronunciation);
    map.insert(QStringLiteral("pronunciationStyle"), pronunciationStyleToString(dictionary.pronunciationStyle));

    map.insert(QStringLiteral("cacheEnabled"), performance.cacheEnabled);
    map.insert(QStringLiteral("lazyInitialize"), performance.lazyInitialize);
    return map;
}

} // namespace Lexicon

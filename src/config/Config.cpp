#include "config/Config.h"

#include <QColor>
#include <QPalette>
#include <QStandardPaths>

namespace {

QString colorString(const QColor& color) {
    return color.alpha() == 255 ? color.name(QColor::HexRgb) : color.name(QColor::HexArgb);
}

void addPaletteColor(QVariantMap& colors, const char* name, const QColor& color) {
    if (color.isValid()) {
        colors.insert(QString::fromLatin1(name), colorString(color));
    }
}

} // namespace

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
    case PronunciationStyle::Ipa:
        return QStringLiteral("ipa");
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

QString Config::resolvedDictionaryDirectory() const {
    if (!dictionary.directory.trimmed().isEmpty()) {
        return dictionary.directory;
    }
    return QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation) +
           QStringLiteral("/lexicon/dictionaries");
}

QStringList Config::resolvedDictionaryDirectories() const {
    if (!dictionary.directory.trimmed().isEmpty()) {
        return {dictionary.directory};
    }
    QStringList dirs;
    const QString subPath = QStringLiteral("/lexicon/dictionaries");
    const QStringList dataDirs = QStandardPaths::standardLocations(QStandardPaths::GenericDataLocation);
    for (const QString& base : dataDirs) {
        dirs.append(base + subPath);
    }
    return dirs;
}

bool Config::usesDictionaryDirectory() const {
    return dictionary.databasePath.trimmed().isEmpty();
}

QKeySequence Config::lookupKeySequence() const {
    return parseKeySequence(hotkeys.lookup);
}

bool resolveDarkPalette(Theme theme, Qt::ColorScheme systemScheme) {
    switch (theme) {
    case Theme::Dark:
        return true;
    case Theme::Light:
        return false;
    case Theme::System:
        return systemScheme != Qt::ColorScheme::Light;
    }
    return true;
}

QVariantMap Config::toVariantMap(Qt::ColorScheme systemScheme, const QPalette* systemPalette) const {
    QVariantMap map;
    map.insert(QStringLiteral("startOnLogin"), general.startOnLogin);
    map.insert(QStringLiteral("offlineFirst"), general.offlineFirst);

    map.insert(QStringLiteral("activation"), activationToString(interaction.activation));
    map.insert(QStringLiteral("clipboardFallback"), interaction.clipboardFallback);
    map.insert(QStringLiteral("dismissOnEscape"), interaction.dismissOnEscape);
    map.insert(QStringLiteral("dismissOnOutsideClick"), interaction.dismissOnOutsideClick);

    map.insert(QStringLiteral("lookup"), hotkeys.lookup);

    map.insert(QStringLiteral("theme"), themeToString(appearance.theme));
    map.insert(QStringLiteral("themeUsesDarkPalette"), resolveDarkPalette(appearance.theme, systemScheme));
    map.insert(QStringLiteral("popupWidth"), appearance.popupWidth);
    map.insert(QStringLiteral("animation"), animationToString(appearance.animation));
    map.insert(QStringLiteral("position"), popupPositionToString(appearance.position));
    map.insert(QStringLiteral("fontFamily"), appearance.fontFamily);
    map.insert(QStringLiteral("cornerRadius"), appearance.cornerRadius);

    QVariantMap themeColors;
    if (appearance.theme == Theme::System && systemPalette != nullptr) {
        addPaletteColor(themeColors, "background", systemPalette->color(QPalette::Window));
        addPaletteColor(themeColors, "surface", systemPalette->color(QPalette::Base));
        addPaletteColor(themeColors, "surfaceRaised", systemPalette->color(QPalette::Button));
        addPaletteColor(themeColors, "border", systemPalette->color(QPalette::Mid));
        addPaletteColor(themeColors, "textPrimary", systemPalette->color(QPalette::Text));
        addPaletteColor(themeColors, "textSecondary", systemPalette->color(QPalette::PlaceholderText));
        addPaletteColor(themeColors, "accent", systemPalette->color(QPalette::Highlight));
    }

    const auto addColor = [&themeColors](const char* name, const QString& value) {
        if (!value.isEmpty()) {
            themeColors.insert(QString::fromLatin1(name), value);
        }
    };
    addColor("background", appearance.colors.background);
    addColor("surface", appearance.colors.surface);
    addColor("surfaceRaised", appearance.colors.surfaceRaised);
    addColor("border", appearance.colors.border);
    addColor("textPrimary", appearance.colors.textPrimary);
    addColor("textSecondary", appearance.colors.textSecondary);
    addColor("accent", appearance.colors.accent);
    addColor("danger", appearance.colors.danger);
    map.insert(QStringLiteral("themeColors"), themeColors);

    map.insert(QStringLiteral("databasePath"), resolvedDatabasePath());
    map.insert(QStringLiteral("dictionaryDirectory"), resolvedDictionaryDirectory());
    map.insert(QStringLiteral("sourceLanguage"), dictionary.sourceLanguage);
    map.insert(QStringLiteral("targetLanguage"), dictionary.targetLanguage);
    map.insert(QStringLiteral("showPronunciation"), dictionary.showPronunciation);
    map.insert(QStringLiteral("pronunciationStyle"), pronunciationStyleToString(dictionary.pronunciationStyle));

    map.insert(QStringLiteral("languagePackCatalogUrl"), translation.catalogUrl);
    map.insert(QStringLiteral("languagePacksConfigured"), !translation.catalogUrl.isEmpty());

    map.insert(QStringLiteral("cacheEnabled"), performance.cacheEnabled);
    map.insert(QStringLiteral("cacheSize"), performance.cacheSize);
    map.insert(QStringLiteral("lazyInitialize"), performance.lazyInitialize);
    return map;
}

} // namespace Lexicon

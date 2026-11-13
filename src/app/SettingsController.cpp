#include "app/SettingsController.h"

#include "app/ApplicationController.h"

#include <QGuiApplication>
#include <QStyleHints>

namespace Lexicon {

SettingsController::SettingsController(Config config, ApplicationController* controller, QObject* parent)
    : QObject(parent), m_config(std::move(config)), m_controller(controller) {}

void SettingsController::setStatus(const QString& status) {
    if (m_status == status) {
        return;
    }
    m_status = status;
    Q_EMIT statusChanged();
}

void SettingsController::setSaving(bool saving) {
    if (m_saving == saving) {
        return;
    }
    m_saving = saving;
    Q_EMIT savingChanged();
}

void SettingsController::applyAutostart() {
    if (m_controller) {
        m_controller->setStartOnLogin(m_config.general.startOnLogin);
    }
}

void SettingsController::setStartOnLogin(bool value) {
    if (m_config.general.startOnLogin == value) {
        return;
    }
    m_config.general.startOnLogin = value;
    applyAutostart();
    Q_EMIT startOnLoginChanged();
}

void SettingsController::setOfflineFirst(bool value) {
    if (m_config.general.offlineFirst == value) {
        return;
    }
    m_config.general.offlineFirst = value;
    Q_EMIT offlineFirstChanged();
}

void SettingsController::setDismissOnEscape(bool value) {
    if (m_config.interaction.dismissOnEscape == value) {
        return;
    }
    m_config.interaction.dismissOnEscape = value;
    Q_EMIT dismissOnEscapeChanged();
}

void SettingsController::setDismissOnOutsideClick(bool value) {
    if (m_config.interaction.dismissOnOutsideClick == value) {
        return;
    }
    m_config.interaction.dismissOnOutsideClick = value;
    Q_EMIT dismissOnOutsideClickChanged();
}

void SettingsController::setLookup(const QString& value) {
    if (m_config.hotkeys.lookup == value) {
        return;
    }
    m_config.hotkeys.lookup = value;
    Q_EMIT lookupChanged();
}

void SettingsController::setTheme(const QString& value) {
    Theme parsed = m_config.appearance.theme;
    if (value == QStringLiteral("dark")) {
        parsed = Theme::Dark;
    } else if (value == QStringLiteral("light")) {
        parsed = Theme::Light;
    } else if (value == QStringLiteral("system")) {
        parsed = Theme::System;
    } else {
        return;
    }
    if (m_config.appearance.theme == parsed) {
        return;
    }
    m_config.appearance.theme = parsed;
    Q_EMIT themeChanged();
}

void SettingsController::setPopupWidth(int value) {
    const int clamped = qBound(240, value, 1200);
    if (m_config.appearance.popupWidth == clamped) {
        return;
    }
    m_config.appearance.popupWidth = clamped;
    Q_EMIT popupWidthChanged();
}

void SettingsController::setAnimation(const QString& value) {
    Animation parsed = m_config.appearance.animation;
    if (value == QStringLiteral("none")) {
        parsed = Animation::None;
    } else if (value == QStringLiteral("subtle")) {
        parsed = Animation::Subtle;
    } else {
        return;
    }
    if (m_config.appearance.animation == parsed) {
        return;
    }
    m_config.appearance.animation = parsed;
    Q_EMIT animationChanged();
}

void SettingsController::setPosition(const QString& value) {
    PopupPosition parsed = m_config.appearance.position;
    if (value == QStringLiteral("near_selection")) {
        parsed = PopupPosition::NearSelection;
    } else if (value == QStringLiteral("center")) {
        parsed = PopupPosition::Center;
    } else if (value == QStringLiteral("cursor")) {
        parsed = PopupPosition::Cursor;
    } else {
        return;
    }
    if (m_config.appearance.position == parsed) {
        return;
    }
    m_config.appearance.position = parsed;
    Q_EMIT positionChanged();
}

void SettingsController::setDatabasePath(const QString& value) {
    if (m_config.dictionary.databasePath == value) {
        return;
    }
    m_config.dictionary.databasePath = value;
    Q_EMIT databasePathChanged();
}

void SettingsController::setDictionaryDirectory(const QString& value) {
    if (m_config.dictionary.directory == value) {
        return;
    }
    m_config.dictionary.directory = value;
    Q_EMIT dictionaryDirectoryChanged();
}

void SettingsController::setSourceLanguage(const QString& value) {
    if (m_config.dictionary.sourceLanguage == value) {
        return;
    }
    m_config.dictionary.sourceLanguage = value;
    Q_EMIT sourceLanguageChanged();
}

void SettingsController::setTargetLanguage(const QString& value) {
    if (m_config.dictionary.targetLanguage == value) {
        return;
    }
    m_config.dictionary.targetLanguage = value;
    Q_EMIT targetLanguageChanged();
}

void SettingsController::setShowPronunciation(bool value) {
    if (m_config.dictionary.showPronunciation == value) {
        return;
    }
    m_config.dictionary.showPronunciation = value;
    Q_EMIT showPronunciationChanged();
}

void SettingsController::setPronunciationStyle(const QString& value) {
    PronunciationStyle parsed = m_config.dictionary.pronunciationStyle;
    if (value == QStringLiteral("respelling")) {
        parsed = PronunciationStyle::Respelling;
    } else if (value == QStringLiteral("ipa")) {
        parsed = PronunciationStyle::Ipa;
    } else {
        return;
    }
    if (m_config.dictionary.pronunciationStyle == parsed) {
        return;
    }
    m_config.dictionary.pronunciationStyle = parsed;
    Q_EMIT pronunciationStyleChanged();
}

void SettingsController::setCatalogUrl(const QString& value) {
    if (m_config.translation.catalogUrl == value) {
        return;
    }
    m_config.translation.catalogUrl = value;
    Q_EMIT catalogUrlChanged();
}

void SettingsController::setCacheEnabled(bool value) {
    if (m_config.performance.cacheEnabled == value) {
        return;
    }
    m_config.performance.cacheEnabled = value;
    Q_EMIT cacheEnabledChanged();
}

void SettingsController::setCacheSize(int value) {
    const int clamped = qBound(0, value, 4096);
    if (m_config.performance.cacheSize == clamped) {
        return;
    }
    m_config.performance.cacheSize = clamped;
    Q_EMIT cacheSizeChanged();
}

void SettingsController::setLazyInitialize(bool value) {
    if (m_config.performance.lazyInitialize == value) {
        return;
    }
    m_config.performance.lazyInitialize = value;
    Q_EMIT lazyInitializeChanged();
}

bool SettingsController::save() {
    setSaving(true);
    const bool ok = m_loader.save(m_config, m_config.configPath);
    setSaving(false);
    if (ok) {
        setStatus(QStringLiteral("Saved. Restart Lexicon to apply all changes."));
        Q_EMIT configSaved(m_config.toVariantMap(QGuiApplication::styleHints()->colorScheme()));
    } else {
        setStatus(QStringLiteral("Failed to save configuration."));
    }
    return ok;
}

void SettingsController::reload() {
    ConfigLoadResult result = m_loader.load(m_config.configPath);
    m_config = result.config;
    // The on-disk value may differ from the live daemon state, so reconcile the
    // autostart entry as part of the reload.
    applyAutostart();
    // re-Q_EMIT all
    Q_EMIT startOnLoginChanged();
    Q_EMIT offlineFirstChanged();
    Q_EMIT dismissOnEscapeChanged();
    Q_EMIT dismissOnOutsideClickChanged();
    Q_EMIT lookupChanged();
    Q_EMIT themeChanged();
    Q_EMIT popupWidthChanged();
    Q_EMIT animationChanged();
    Q_EMIT positionChanged();
    Q_EMIT databasePathChanged();
    Q_EMIT dictionaryDirectoryChanged();
    Q_EMIT sourceLanguageChanged();
    Q_EMIT targetLanguageChanged();
    Q_EMIT showPronunciationChanged();
    Q_EMIT pronunciationStyleChanged();
    Q_EMIT catalogUrlChanged();
    Q_EMIT cacheEnabledChanged();
    Q_EMIT cacheSizeChanged();
    Q_EMIT lazyInitializeChanged();
    setStatus(QStringLiteral("Reloaded from disk."));
}

} // namespace Lexicon

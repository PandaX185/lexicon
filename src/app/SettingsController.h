#pragma once

#include "config/Config.h"
#include "config/ConfigLoader.h"

#include <QObject>
#include <QString>
#include <QVariantMap>

namespace Lexicon {

class ApplicationController;

// Exposes the runtime configuration to QML in an editable form and persists
// changes back to config.toml. Settings that need a restart to take effect are
// documented in the UI; the controller still saves them immediately.
class SettingsController final : public QObject {
    Q_OBJECT

    Q_PROPERTY(bool startOnLogin READ startOnLogin WRITE setStartOnLogin NOTIFY startOnLoginChanged)
    Q_PROPERTY(bool offlineFirst READ offlineFirst WRITE setOfflineFirst NOTIFY offlineFirstChanged)
    Q_PROPERTY(bool dismissOnEscape READ dismissOnEscape WRITE setDismissOnEscape NOTIFY dismissOnEscapeChanged)
    Q_PROPERTY(bool dismissOnOutsideClick READ dismissOnOutsideClick WRITE setDismissOnOutsideClick NOTIFY
                   dismissOnOutsideClickChanged)
    Q_PROPERTY(QString lookup READ lookup WRITE setLookup NOTIFY lookupChanged)
    Q_PROPERTY(QString theme READ theme WRITE setTheme NOTIFY themeChanged)
    Q_PROPERTY(int popupWidth READ popupWidth WRITE setPopupWidth NOTIFY popupWidthChanged)
    Q_PROPERTY(QString animation READ animation WRITE setAnimation NOTIFY animationChanged)
    Q_PROPERTY(QString position READ position WRITE setPosition NOTIFY positionChanged)
    Q_PROPERTY(QString databasePath READ databasePath WRITE setDatabasePath NOTIFY databasePathChanged)
    Q_PROPERTY(QString dictionaryDirectory READ dictionaryDirectory WRITE setDictionaryDirectory NOTIFY
                   dictionaryDirectoryChanged)
    Q_PROPERTY(QString sourceLanguage READ sourceLanguage WRITE setSourceLanguage NOTIFY sourceLanguageChanged)
    Q_PROPERTY(QString targetLanguage READ targetLanguage WRITE setTargetLanguage NOTIFY targetLanguageChanged)
    Q_PROPERTY(bool showPronunciation READ showPronunciation WRITE setShowPronunciation NOTIFY showPronunciationChanged)
    Q_PROPERTY(
        QString pronunciationStyle READ pronunciationStyle WRITE setPronunciationStyle NOTIFY pronunciationStyleChanged)
    Q_PROPERTY(QString catalogUrl READ catalogUrl WRITE setCatalogUrl NOTIFY catalogUrlChanged)
    Q_PROPERTY(bool cacheEnabled READ cacheEnabled WRITE setCacheEnabled NOTIFY cacheEnabledChanged)
    Q_PROPERTY(int cacheSize READ cacheSize WRITE setCacheSize NOTIFY cacheSizeChanged)
    Q_PROPERTY(bool lazyInitialize READ lazyInitialize WRITE setLazyInitialize NOTIFY lazyInitializeChanged)
    Q_PROPERTY(QString status READ status NOTIFY statusChanged)
    Q_PROPERTY(bool saving READ saving NOTIFY savingChanged)

  public:
    explicit SettingsController(Config config, ApplicationController* controller = nullptr, QObject* parent = nullptr);

    void setController(ApplicationController* controller) {
        m_controller = controller;
    }
    ~SettingsController() override = default;

    bool startOnLogin() const {
        return m_config.general.startOnLogin;
    }
    bool offlineFirst() const {
        return m_config.general.offlineFirst;
    }
    bool dismissOnEscape() const {
        return m_config.interaction.dismissOnEscape;
    }
    bool dismissOnOutsideClick() const {
        return m_config.interaction.dismissOnOutsideClick;
    }
    QString lookup() const {
        return m_config.hotkeys.lookup;
    }
    QString theme() const {
        return themeToString(m_config.appearance.theme);
    }
    int popupWidth() const {
        return m_config.appearance.popupWidth;
    }
    QString animation() const {
        return animationToString(m_config.appearance.animation);
    }
    QString position() const {
        return popupPositionToString(m_config.appearance.position);
    }
    QString databasePath() const {
        return m_config.dictionary.databasePath;
    }
    QString dictionaryDirectory() const {
        return m_config.dictionary.directory;
    }
    QString sourceLanguage() const {
        return m_config.dictionary.sourceLanguage;
    }
    QString targetLanguage() const {
        return m_config.dictionary.targetLanguage;
    }
    bool showPronunciation() const {
        return m_config.dictionary.showPronunciation;
    }
    QString pronunciationStyle() const {
        return pronunciationStyleToString(m_config.dictionary.pronunciationStyle);
    }
    QString catalogUrl() const {
        return m_config.translation.catalogUrl;
    }
    bool cacheEnabled() const {
        return m_config.performance.cacheEnabled;
    }
    int cacheSize() const {
        return m_config.performance.cacheSize;
    }
    bool lazyInitialize() const {
        return m_config.performance.lazyInitialize;
    }
    QString status() const {
        return m_status;
    }
    bool saving() const {
        return m_saving;
    }

    void setStartOnLogin(bool value);
    void setOfflineFirst(bool value);
    void setDismissOnEscape(bool value);
    void setDismissOnOutsideClick(bool value);
    void setLookup(const QString& value);
    void setTheme(const QString& value);
    void setPopupWidth(int value);
    void setAnimation(const QString& value);
    void setPosition(const QString& value);
    void setDatabasePath(const QString& value);
    void setDictionaryDirectory(const QString& value);
    void setSourceLanguage(const QString& value);
    void setTargetLanguage(const QString& value);
    void setShowPronunciation(bool value);
    void setPronunciationStyle(const QString& value);
    void setCatalogUrl(const QString& value);
    void setCacheEnabled(bool value);
    void setCacheSize(int value);
    void setLazyInitialize(bool value);

    Q_INVOKABLE bool save();
    Q_INVOKABLE void reload();

    // Returns the current config so callers can refresh other components.
    [[nodiscard]] const Config& config() const {
        return m_config;
    }

  Q_SIGNALS:
    void startOnLoginChanged();
    void offlineFirstChanged();
    void dismissOnEscapeChanged();
    void dismissOnOutsideClickChanged();
    void lookupChanged();
    void themeChanged();
    void popupWidthChanged();
    void animationChanged();
    void positionChanged();
    void databasePathChanged();
    void dictionaryDirectoryChanged();
    void sourceLanguageChanged();
    void targetLanguageChanged();
    void showPronunciationChanged();
    void pronunciationStyleChanged();
    void catalogUrlChanged();
    void cacheEnabledChanged();
    void cacheSizeChanged();
    void lazyInitializeChanged();
    void statusChanged();
    void savingChanged();
    void configSaved(const QVariantMap& map);

  private:
    void setStatus(const QString& status);
    void setSaving(bool saving);
    void applyAutostart();

    Config m_config;
    ApplicationController* m_controller;
    ConfigLoader m_loader;
    QString m_status;
    bool m_saving = false;

    Q_DISABLE_COPY_MOVE(SettingsController)
};

} // namespace Lexicon

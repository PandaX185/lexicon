#pragma once

#include "app/AutostartManager.h"
#include "config/Config.h"
#include "dictionary/DictionaryRepository.h"

#include <QFutureWatcher>
#include <QObject>

#include <memory>

namespace Lexicon {

class DictionaryService;
class PopupPresenter;
class SelectionProvider;
class ShortcutService;
class TranslationService;
class TrayService;

class ApplicationController final : public QObject {
    Q_OBJECT

  public:
    explicit ApplicationController(Config config, std::unique_ptr<DictionaryService> dictionary,
                                   std::unique_ptr<TranslationService> translation,
                                   std::unique_ptr<SelectionProvider> selection,
                                   std::unique_ptr<ShortcutService> shortcuts, std::unique_ptr<TrayService> tray,
                                   PopupPresenter& presenter,
                                   AutostartManager autostart = AutostartManager::forCurrentUser(),
                                   QObject* parent = nullptr);
    ~ApplicationController() override;

    void start();

    // Reconciles the per-user autostart entry with the current configuration.
    // Safe to call after updating `config` at runtime.
    void syncAutostart();

    // Updates the daemon's live config and reconciles the autostart entry.
    void setStartOnLogin(bool enabled);

  public Q_SLOTS:
    void lookupSelection();
    void lookupWord(const QString& word);
    void showSettings();
    void quit();

  private:
    void handleLookupFinished();

    Config m_config;
    std::unique_ptr<DictionaryService> m_dictionary;
    // Optional: the dictionary works without a translation backend.
    std::unique_ptr<TranslationService> m_translation;
    std::unique_ptr<SelectionProvider> m_selection;
    std::unique_ptr<ShortcutService> m_shortcuts;
    std::unique_ptr<TrayService> m_tray;
    PopupPresenter& m_presenter;
    QFutureWatcher<DictionaryLookupResult> m_watcher;
    AutostartManager m_autostart;

    Q_DISABLE_COPY(ApplicationController);

    ApplicationController(ApplicationController&&) noexcept = delete;
    ApplicationController& operator=(ApplicationController&&) noexcept = delete;
};

} // namespace Lexicon

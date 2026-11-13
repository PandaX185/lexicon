#include "app/ApplicationController.h"

#include "core/interfaces/PopupPresenter.h"
#include "core/interfaces/SelectionProvider.h"
#include "core/interfaces/ShortcutService.h"
#include "core/interfaces/TranslationService.h"
#include "core/interfaces/TrayService.h"
#include "dictionary/DictionaryService.h"
#include "infrastructure/logging/Logging.h"

#include <QCoreApplication>
#include <QFutureWatcher>
#include <QtConcurrent/QtConcurrentRun>

#include <memory>
#include <utility>

namespace Lexicon {
namespace {

// Selection failures are distinct on purpose; the popup title should say which
// one happened instead of reporting every case as "No selection".
QString selectionErrorTitle(ErrorCode code) {
    switch (code) {
    case ErrorCode::SelectionNotText:
        return QStringLiteral("Selection is not text");
    case ErrorCode::SelectionTooLong:
        return QStringLiteral("Selection too long");
    case ErrorCode::SelectionUnsupported:
        return QStringLiteral("Selection not supported");
    case ErrorCode::SelectionUnavailable:
    case ErrorCode::None:
    case ErrorCode::InvalidArgument:
    case ErrorCode::ConfigParseError:
    case ErrorCode::ConfigValidationError:
    case ErrorCode::DatabaseUnavailable:
    case ErrorCode::DatabaseOpenFailed:
    case ErrorCode::DatabaseQueryFailed:
    case ErrorCode::WordNotFound:
    case ErrorCode::TranslationUnavailable:
    case ErrorCode::ProtocolViolation:
    case ErrorCode::ShortcutRegistrationFailed:
    case ErrorCode::TrayUnavailable:
    case ErrorCode::Unknown:
        break;
    }
    return QStringLiteral("No selection");
}

} // namespace

ApplicationController::ApplicationController(Config config, std::unique_ptr<DictionaryService> dictionary,
                                             std::unique_ptr<TranslationService> translation,
                                             std::unique_ptr<SelectionProvider> selection,
                                             std::unique_ptr<ShortcutService> shortcuts,
                                             std::unique_ptr<TrayService> tray, PopupPresenter& presenter,
                                             AutostartManager autostart, QObject* parent)
    : QObject(parent), m_config(std::move(config)), m_dictionary(std::move(dictionary)),
      m_translation(std::move(translation)), m_selection(std::move(selection)), m_shortcuts(std::move(shortcuts)),
      m_tray(std::move(tray)), m_presenter(presenter), m_autostart(std::move(autostart)) {
    connect(&m_watcher, &QFutureWatcherBase::finished, this, &ApplicationController::handleLookupFinished);
}

ApplicationController::~ApplicationController() {
    m_tray->stop();
    m_shortcuts->unregisterAll();

    m_watcher.disconnect();
    if (m_watcher.isRunning()) {
        m_watcher.cancel();
        m_watcher.waitForFinished();
    }
}

void ApplicationController::setStartOnLogin(bool enabled) {
    if (m_config.general.startOnLogin == enabled) {
        return;
    }
    m_config.general.startOnLogin = enabled;
    syncAutostart();
}

void ApplicationController::syncAutostart() {
    const bool applied = m_config.general.startOnLogin ? m_autostart.install(QCoreApplication::applicationFilePath())
                                                       : m_autostart.remove();
    if (!applied) {
        qCWarning(lexiconApp) << "event=autostart_sync_failed enabled=" << m_config.general.startOnLogin;
    }
}

void ApplicationController::start() {
    const ShortcutSpec shortcut{
        .id = QStringLiteral("lookup_selection"),
        .displayName = QStringLiteral("Look up selected text"),
        .defaultSequence = m_config.hotkeys.lookup,
    };
    const ServiceStatus shortcutStatus = m_shortcuts->registerShortcut(shortcut, [this] { lookupSelection(); });
    if (!shortcutStatus.available) {
        qCWarning(lexiconApp) << "event=shortcut_unavailable message=" << shortcutStatus.error.message;
    }

    const TrayActions trayActions{
        .openSettings = [this] { showSettings(); },
        .lookupSelection = [this] { lookupSelection(); },
        .quit = [this] { quit(); },
    };

    syncAutostart();
    if (m_config.general.startOnLogin) {
        qCInfo(lexiconApp) << "event=start_on_login_enabled path=" << m_autostart.targetPath();
    }
    const ServiceStatus trayStatus = m_tray->start(trayActions);
    if (!trayStatus.available) {
        qCWarning(lexiconApp) << "event=tray_unavailable message=" << trayStatus.error.message;
    }

    qCInfo(lexiconApp) << "event=application_started database=" << m_dictionary->repositoryLocation()
                       << "selection=" << m_selection->displayName();
}

void ApplicationController::lookupSelection() {
    const SelectionResult selection = m_selection->currentSelection();
    const QString selectedText = selection.text.value_or(QString());
    if (selectedText.trimmed().isEmpty()) {
        QString message = selection.error.message;
        if (message.isEmpty()) {
            message = QStringLiteral("No text is currently selected.");
        }
        if (!selection.error.detail.isEmpty()) {
            message += QStringLiteral("\n\n") + selection.error.detail;
        }
        m_presenter.showMessage(selectionErrorTitle(selection.error.code), message);
        return;
    }

    lookupWord(selectedText);
}

void ApplicationController::lookupWord(const QString& word) {
    const QString trimmed = word.trimmed();
    if (trimmed.isEmpty()) {
        m_presenter.showMessage(QStringLiteral("Nothing to look up"),
                                QStringLiteral("Select a word or enter one to look up."));
        return;
    }

    if (m_watcher.isRunning()) {
        qCInfo(lexiconApp) << "event=lookup_ignored reason=already_running";
        return;
    }

    m_presenter.showLoading(trimmed);

    DictionaryService* service = m_dictionary.get();
    TranslationService* translation = m_translation.get();
    const QString sourceLanguage = m_config.dictionary.sourceLanguage;
    const QString targetLanguage = m_config.dictionary.targetLanguage;

    // The dictionary answer stands on its own: a missing or failed translation
    // backend leaves the entry untouched instead of failing the lookup.
    m_watcher.setFuture(QtConcurrent::run([service, translation, trimmed, sourceLanguage, targetLanguage] {
        DictionaryLookupResult result = service->lookup(trimmed);
        if (result.found() && translation != nullptr && sourceLanguage != targetLanguage) {
            const Result<QString> translated = translation->translate(trimmed, sourceLanguage, targetLanguage);
            if (translated.hasValue()) {
                result.entry->metadata.insert(QStringLiteral("translation"), translated.value());
            } else {
                qCInfo(lexiconApp) << "event=translation_unavailable error="
                                   << errorCodeToString(translated.error().code);
            }
        }
        return result;
    }));
}

void ApplicationController::showSettings() {
    m_presenter.showSettings();
}

void ApplicationController::quit() {
    QCoreApplication::quit();
}

void ApplicationController::handleLookupFinished() {
    const DictionaryLookupResult result = m_watcher.result();

    if (result.error.isError()) {
        qCInfo(lexiconApp) << "event=lookup_completed found=false error=" << errorCodeToString(result.error.code);
        if (result.error.code == ErrorCode::WordNotFound) {
            m_presenter.showMessage(QStringLiteral("No entry found"), result.error.message);
            return;
        }

        QString message = result.error.message;
        if (!result.error.detail.isEmpty()) {
            message += QStringLiteral("\n\n") + result.error.detail;
        }
        m_presenter.showMessage(QStringLiteral("Lookup failed"), message);
        return;
    }

    if (!result.found()) {
        m_presenter.showMessage(QStringLiteral("No entry found"), QStringLiteral("The dictionary returned no result."));
        return;
    }

    qCInfo(lexiconApp) << "event=lookup_completed found=true definitions=" << result.entry->definitions.size();
    m_presenter.showEntry(*result.entry);
}

} // namespace Lexicon

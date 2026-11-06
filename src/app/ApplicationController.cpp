#include "app/ApplicationController.h"

#include "core/interfaces/PopupPresenter.h"
#include "core/interfaces/SelectionProvider.h"
#include "core/interfaces/ShortcutService.h"
#include "core/interfaces/TrayService.h"
#include "dictionary/DictionaryService.h"
#include "infrastructure/logging/Logging.h"

#include <QCoreApplication>
#include <QFutureWatcher>
#include <QStandardPaths>
#include <QtConcurrent/QtConcurrentRun>

#include <memory>
#include <utility>

namespace Lexicon {

ApplicationController::ApplicationController(Config config, std::unique_ptr<DictionaryService> dictionary,
                                             std::unique_ptr<SelectionProvider> selection,
                                             std::unique_ptr<ShortcutService> shortcuts,
                                             std::unique_ptr<TrayService> tray, PopupPresenter& presenter,
                                             QObject* parent)
    : QObject(parent), m_config(std::move(config)), m_dictionary(std::move(dictionary)),
      m_selection(std::move(selection)), m_shortcuts(std::move(shortcuts)), m_tray(std::move(tray)),
      m_presenter(presenter) {
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

QString ApplicationController::installedAutostartFilePath() {
    return QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation) +
           QStringLiteral("/autostart/org.lexicon.Lexicon.desktop");
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

    if (m_config.general.startOnLogin) {
        qCInfo(lexiconApp) << "event=start_on_login_requested action=install_autostart_entry"
                           << "source=" << installedAutostartFilePath() << "stage=1 note=install_not_implemented";
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
        if (!selection.error.detail.isEmpty()) {
            message += QStringLiteral("\n\n") + selection.error.detail;
        }
        m_presenter.showMessage(QStringLiteral("No selection"), message);
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
    m_watcher.setFuture(QtConcurrent::run([service, trimmed] { return service->lookup(trimmed); }));
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

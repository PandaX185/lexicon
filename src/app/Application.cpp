#include "app/Application.h"

#include "app/ApplicationController.h"
#include "app/DictionarySeeder.h"
#include "app/LanguagePackController.h"
#include "app/QmlPresenter.h"
#include "app/SelectionProviderFactory.h"
#include "app/SettingsController.h"
#include "core/interfaces/SelectionProvider.h"
#include "dictionary/CatalogDictionaryRepository.h"
#include "dictionary/DictionaryCatalog.h"
#include "dictionary/DictionaryService.h"
#include "infrastructure/UnixSignalHandler.h"
#include "infrastructure/database/SqliteDictionaryRepository.h"
#include "infrastructure/logging/Logging.h"
#include "infrastructure/translation/ArgosTranslationService.h"
#include "kde/session/KdeSession.h"
#include "kde/shortcuts/KGlobalAccelShortcutService.h"
#include "kde/tray/KStatusNotifierTrayService.h"

#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QStyleHints>
#include <QTimer>

namespace {

QVariantMap buildConfigMap(const Lexicon::Config& config) {
    return config.toVariantMap(QGuiApplication::styleHints()->colorScheme());
}

// A dictionary can be pointed at explicitly, which keeps single-file setups and
// the test fixture working; otherwise the installed dictionaries are searched
// for the configured language pair.
std::unique_ptr<Lexicon::DictionaryRepository> buildDictionaryRepository(const Lexicon::Config& config) {
    const auto open = [](const QString& path) -> std::unique_ptr<Lexicon::DictionaryRepository> {
        return std::make_unique<Lexicon::SqliteDictionaryRepository>(path);
    };

    if (!config.usesDictionaryDirectory()) {
        return open(config.resolvedDatabasePath());
    }

    return std::make_unique<Lexicon::CatalogDictionaryRepository>(
        Lexicon::DictionaryCatalog(config.resolvedDictionaryDirectories(), open), open,
        config.dictionary.sourceLanguage, config.dictionary.targetLanguage);
}

} // namespace

namespace Lexicon {

Application::Application(Config config) : m_config(std::move(config)) {
    m_engine = std::make_unique<QQmlApplicationEngine>();
    m_engine->rootContext()->setContextProperty(QStringLiteral("config"), buildConfigMap(m_config));
    m_engine->rootContext()->setContextProperty(QStringLiteral("appVersion"), QStringLiteral("0.1.0"));

    QObject::connect(QGuiApplication::styleHints(), &QStyleHints::colorSchemeChanged, m_engine.get(), [this] {
        m_engine->rootContext()->setContextProperty(QStringLiteral("config"), buildConfigMap(m_config));
    });

    // The settings window drives language packs through this object; it only
    // downloads when the user presses install. It has to exist before the QML
    // is loaded, because the settings view binds to it.
    m_languagePacks = std::make_unique<LanguagePackController>(m_config);
    m_engine->rootContext()->setContextProperty(QStringLiteral("languagePacks"), m_languagePacks.get());

    // Editable settings: created before QML load so the settings view can bind
    // to it; the controller pointer is attached once ApplicationController
    // exists.
    m_settings = std::make_unique<SettingsController>(m_config, nullptr);
    m_engine->rootContext()->setContextProperty(QStringLiteral("settings"), m_settings.get());

    if (Lexicon::seedUserDictionary(m_config)) {
        qCInfo(lexiconApp) << "event=dictionary_first_run_seeded";
    }

    m_engine->load(QUrl(QStringLiteral("qrc:/ui/Main.qml")));
    if (m_engine->rootObjects().isEmpty()) {
        qCCritical(lexiconUi) << "event=qml_load_failed path=qrc:/ui/Main.qml";
    } else {
        m_qmlReady = true;
    }

    m_presenter = std::make_unique<QmlPresenter>(*m_engine, m_config);

    auto dictionary = std::make_unique<DictionaryService>(
        buildDictionaryRepository(m_config), m_config.performance.cacheEnabled ? m_config.performance.cacheSize : 0);
    m_dictionaryAvailable = !dictionary->repositoryLocation().isEmpty();
    if (!m_dictionaryAvailable) {
        qCWarning(lexiconApp) << "event=dictionary_unavailable hint=install_dictionary_data";
    }

    m_session = std::make_unique<KdeSession>();
    m_signals = std::make_unique<UnixSignalHandler>();

    // Translation is optional and only reachable when a language pair is
    // configured; the worker process starts on the first translated lookup.
    m_controller = std::make_unique<ApplicationController>(
        m_config, std::move(dictionary), std::make_unique<ArgosTranslationService>(), createSelectionProvider(),
        std::make_unique<KGlobalAccelShortcutService>(), std::make_unique<KStatusNotifierTrayService>(), *m_presenter);

    if (m_settings) {
        m_settings->setController(m_controller.get());
        QObject::connect(m_settings.get(), &SettingsController::configSaved, m_engine.get(),
                         [this](const QVariantMap& map) {
                             m_engine->rootContext()->setContextProperty(QStringLiteral("config"), map);
                         });
    }

    QObject::connect(m_signals.get(), &UnixSignalHandler::terminationRequested, m_controller.get(),
                     &ApplicationController::quit);
    QObject::connect(m_session.get(), &KdeSession::activationRequested, m_controller.get(),
                     &ApplicationController::showSettings);
}

Application::~Application() = default;

int Application::run(const QString& initialLookup) {
    if (!m_qmlReady) {
        return 1;
    }

    m_controller->start();
    if (!m_dictionaryAvailable) {
        m_presenter->showMessage(
            QStringLiteral("No dictionary installed"),
            QStringLiteral("Lexicon could not find a dictionary for the configured language pair. "
                           "Install the package data or run `just install-dictionary` for development builds."));
    }
    if (!initialLookup.isEmpty()) {
        QTimer::singleShot(0, m_controller.get(),
                           [controller = m_controller.get(), initialLookup] { controller->lookupWord(initialLookup); });
    }

    return QGuiApplication::exec();
}

} // namespace Lexicon

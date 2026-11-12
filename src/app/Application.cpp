#include "app/Application.h"

#include "app/ApplicationController.h"
#include "app/LanguagePackController.h"
#include "app/QmlPresenter.h"
#include "app/SelectionProviderFactory.h"
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

    m_engine->load(QUrl(QStringLiteral("qrc:/ui/Main.qml")));
    if (m_engine->rootObjects().isEmpty()) {
        qCCritical(lexiconUi) << "event=qml_load_failed path=qrc:/ui/Main.qml";
    } else {
        m_qmlReady = true;
    }

    m_presenter = std::make_unique<QmlPresenter>(*m_engine, m_config);

    auto dictionary = std::make_unique<DictionaryService>(buildDictionaryRepository(m_config));

    m_session = std::make_unique<KdeSession>();
    m_signals = std::make_unique<UnixSignalHandler>();

    // Translation is optional and only reachable when a language pair is
    // configured; the worker process starts on the first translated lookup.
    m_controller = std::make_unique<ApplicationController>(
        m_config, std::move(dictionary), std::make_unique<ArgosTranslationService>(), createSelectionProvider(),
        std::make_unique<KGlobalAccelShortcutService>(), std::make_unique<KStatusNotifierTrayService>(), *m_presenter);

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
    if (!initialLookup.isEmpty()) {
        QTimer::singleShot(0, m_controller.get(),
                           [controller = m_controller.get(), initialLookup] { controller->lookupWord(initialLookup); });
    }

    return QGuiApplication::exec();
}

} // namespace Lexicon

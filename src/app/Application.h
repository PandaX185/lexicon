#pragma once

#include "config/Config.h"

#include <QString>

#include <memory>

class QQmlApplicationEngine;

namespace Lexicon {

class ApplicationController;
class KdeSession;
class LanguagePackController;
class QmlPresenter;
class SettingsController;
class UnixSignalHandler;

class Application final {
  public:
    explicit Application(Config config);
    ~Application();

    int run(const QString& initialLookup = {});

  private:
    Config m_config;
    // Declared before the engine so it is destroyed after the QML objects that
    // bind to it; otherwise those bindings evaluate against a null context
    // property during shutdown.
    std::unique_ptr<LanguagePackController> m_languagePacks;
    std::unique_ptr<SettingsController> m_settings;
    std::unique_ptr<QQmlApplicationEngine> m_engine;
    std::unique_ptr<QmlPresenter> m_presenter;
    std::unique_ptr<ApplicationController> m_controller;
    bool m_dictionaryAvailable = true;
    std::unique_ptr<KdeSession> m_session;
    std::unique_ptr<UnixSignalHandler> m_signals;
    bool m_qmlReady = false;
};

} // namespace Lexicon

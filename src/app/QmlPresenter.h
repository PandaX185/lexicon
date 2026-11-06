#pragma once

#include "config/Config.h"
#include "core/interfaces/PopupPresenter.h"

#include <QVariant>

#include <memory>

class QQmlApplicationEngine;
class QQuickView;

namespace Lexicon {

struct WordEntry;

class QmlPresenter final : public PopupPresenter {
  public:
    QmlPresenter(QQmlApplicationEngine& engine, Config config);
    ~QmlPresenter() override;

    void showLoading(const QString& word) override;
    void showEntry(const WordEntry& entry) override;
    void showMessage(const QString& title, const QString& message) override;
    void showSettings() override;
    void dismiss() override;

  private:
    void ensurePopup();
    void setPopupProperty(const char* name, const QVariant& value);
    void positionPopup();
    void presentPopup();

    QQmlApplicationEngine& m_engine;
    Config m_config;
    std::unique_ptr<QQuickView> m_popup;
};

} // namespace Lexicon

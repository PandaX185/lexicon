#pragma once

#include <KDBusService>
#include <QObject>

#include <memory>

namespace Lexicon {

class KdeSession final : public QObject {
    Q_OBJECT

  public:
    explicit KdeSession(QObject* parent = nullptr);
    ~KdeSession() override;

    [[nodiscard]] bool isActive() const;

  Q_SIGNALS:
    void activationRequested();

  private:
    std::unique_ptr<KDBusService> m_service;
};

} // namespace Lexicon

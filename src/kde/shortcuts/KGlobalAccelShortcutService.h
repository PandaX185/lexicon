#pragma once

#include "core/interfaces/ShortcutService.h"

#include <QHash>
#include <QObject>

class QAction;

namespace Lexicon {

class KGlobalAccelShortcutService final : public ShortcutService {
  public:
    ~KGlobalAccelShortcutService() override;

    ServiceStatus registerShortcut(const ShortcutSpec& spec, std::function<void()> callback) override;
    void unregisterShortcut(const QString& id) override;
    void unregisterAll() override;

  private:
    struct Registration {
        QAction* action = nullptr;
        QMetaObject::Connection connection;
    };

    QHash<QString, Registration> m_registrations;
};

} // namespace Lexicon

#include "kde/session/KdeSession.h"

#include "infrastructure/logging/Logging.h"

#include <KDBusService>
#include <QDBusConnection>

namespace Lexicon {

KdeSession::KdeSession(QObject* parent) : QObject(parent) {
    if (!QDBusConnection::sessionBus().isConnected()) {
        qCWarning(lexiconKde) << "event=session_bus_unavailable single_instance_disabled=true";
        return;
    }

    m_service = std::make_unique<KDBusService>(KDBusService::Unique);
    QObject::connect(m_service.get(), &KDBusService::activateRequested, this,
                     [this](const QStringList&, const QString&) { Q_EMIT activationRequested(); });
    qCInfo(lexiconKde) << "event=single_instance_active";
}

KdeSession::~KdeSession() = default;

bool KdeSession::isActive() const {
    return m_service != nullptr;
}

} // namespace Lexicon

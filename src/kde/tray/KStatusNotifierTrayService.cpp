#include "kde/tray/KStatusNotifierTrayService.h"

#include "infrastructure/logging/Logging.h"

#include <KStatusNotifierItem>
#include <QIcon>
#include <QSystemTrayIcon>

namespace Lexicon {

KStatusNotifierTrayService::~KStatusNotifierTrayService() {
    stop();
}

ServiceStatus KStatusNotifierTrayService::start(const TrayActions& actions) {
    if (m_item) {
        return ServiceStatus::success();
    }

    if (!QSystemTrayIcon::isSystemTrayAvailable()) {
        qCWarning(lexiconKde) << "event=tray_unavailable reason=no_system_tray_host";
        return ServiceStatus::failure(AppError::make(
            ErrorCode::TrayUnavailable, QStringLiteral("No system tray host is available in this session.")));
    }

    m_item = std::make_unique<KStatusNotifierItem>(QStringLiteral("org.lexicon.Lexicon"), nullptr);
    m_item->setTitle(QStringLiteral("Lexicon"));
    m_item->setCategory(KStatusNotifierItem::ApplicationStatus);
    m_item->setStatus(KStatusNotifierItem::Active);
    m_item->setIconByPixmap(QIcon(QStringLiteral(":/resources/icons/lexicon.svg")));
    m_item->setToolTip(QStringLiteral("lexicon"), QStringLiteral("Lexicon"),
                       QStringLiteral("Offline English dictionary"));
    m_item->setStandardActionsEnabled(false);

    QObject::connect(m_item.get(), &KStatusNotifierItem::activateRequested, m_item.get(),
                     [actions](bool active, const QPoint&) {
                         if (active && actions.openSettings) {
                             actions.openSettings();
                         }
                     });
    QObject::connect(m_item.get(), &KStatusNotifierItem::secondaryActivateRequested, m_item.get(),
                     [actions](const QPoint&) {
                         if (actions.lookupSelection) {
                             actions.lookupSelection();
                         }
                     });
    QObject::connect(m_item.get(), &KStatusNotifierItem::quitRequested, m_item.get(), [actions] {
        if (actions.quit) {
            actions.quit();
        }
    });

    qCInfo(lexiconKde) << "event=tray_started";
    return ServiceStatus::success();
}

void KStatusNotifierTrayService::stop() {
    if (!m_item) {
        return;
    }
    m_item.reset();
    qCInfo(lexiconKde) << "event=tray_stopped";
}

} // namespace Lexicon

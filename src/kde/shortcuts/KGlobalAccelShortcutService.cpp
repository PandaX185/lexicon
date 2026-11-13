#include "kde/shortcuts/KGlobalAccelShortcutService.h"

#include "config/Config.h"
#include "infrastructure/logging/Logging.h"

#include <KGlobalAccel>
#include <QAction>
#include <QIcon>

#include <utility>

namespace Lexicon {

KGlobalAccelShortcutService::~KGlobalAccelShortcutService() {
    unregisterAll();
}

ServiceStatus KGlobalAccelShortcutService::registerShortcut(const ShortcutSpec& spec, std::function<void()> callback) {
    if (spec.id.isEmpty()) {
        return ServiceStatus::failure(
            AppError::make(ErrorCode::InvalidArgument, QStringLiteral("A shortcut id is required.")));
    }

    unregisterShortcut(spec.id);

    const QKeySequence sequence = parseKeySequence(spec.defaultSequence);
    if (sequence.isEmpty()) {
        return ServiceStatus::failure(AppError::make(
            ErrorCode::InvalidArgument, QStringLiteral("Invalid shortcut sequence: %1").arg(spec.defaultSequence)));
    }

    auto* action = new QAction(nullptr);
    action->setObjectName(spec.id);
    action->setText(spec.displayName);
    action->setIcon(QIcon::fromTheme(QStringLiteral("lexicon")));
    action->setProperty("componentName", QStringLiteral(LEXICON_APP_ID));
    action->setProperty("componentDisplayName", QStringLiteral("Lexicon"));

    const QMetaObject::Connection connection =
        QObject::connect(action, &QAction::triggered, action, [callback = std::move(callback)] {
            if (callback) {
                callback();
            }
        });

    KGlobalAccel* accel = KGlobalAccel::self();
    const bool defaultSet = accel->setDefaultShortcut(action, QList<QKeySequence>{sequence});
    const bool shortcutSet = accel->setShortcut(action, QList<QKeySequence>{sequence}, KGlobalAccel::Autoloading);

    if (!shortcutSet) {
        QObject::disconnect(connection);
        delete action;
        qCWarning(lexiconKde) << "event=shortcut_registration_failed id=" << spec.id
                              << "sequence=" << sequence.toString(QKeySequence::PortableText);
        return ServiceStatus::failure(
            AppError::make(ErrorCode::ShortcutRegistrationFailed,
                           QStringLiteral("KGlobalAccel could not register the shortcut \"%1\".")
                               .arg(sequence.toString(QKeySequence::NativeText))));
    }

    if (!defaultSet) {
        qCWarning(lexiconKde) << "event=shortcut_default_not_set id=" << spec.id;
    }

    m_registrations.insert(spec.id, Registration{.action = action, .connection = connection});
    qCInfo(lexiconKde) << "event=shortcut_registered id=" << spec.id
                       << "sequence=" << sequence.toString(QKeySequence::PortableText);
    return ServiceStatus::success();
}

void KGlobalAccelShortcutService::unregisterShortcut(const QString& id) {
    const auto it = m_registrations.find(id);
    if (it == m_registrations.end()) {
        return;
    }

    if (it->action) {
        KGlobalAccel::self()->removeAllShortcuts(it->action);
        QObject::disconnect(it->connection);
        delete it->action;
    }
    m_registrations.erase(it);
}

void KGlobalAccelShortcutService::unregisterAll() {
    const QStringList ids = m_registrations.keys();
    for (const QString& id : ids) {
        unregisterShortcut(id);
    }
}

} // namespace Lexicon

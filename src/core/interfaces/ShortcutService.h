#pragma once

#include "core/types/ServiceStatus.h"

#include <QString>

#include <functional>

namespace Lexicon {

struct ShortcutSpec {
    QString id;
    QString displayName;
    QString defaultSequence;
};

class ShortcutService {
  public:
    virtual ~ShortcutService() = default;

    virtual ServiceStatus registerShortcut(const ShortcutSpec& spec, std::function<void()> callback) = 0;
    virtual void unregisterShortcut(const QString& id) = 0;
    virtual void unregisterAll() = 0;
};

} // namespace Lexicon

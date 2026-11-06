#pragma once

#include "core/types/ServiceStatus.h"

#include <functional>

namespace Lexicon {

struct TrayActions {
    std::function<void()> openSettings;
    std::function<void()> lookupSelection;
    std::function<void()> quit;
};

class TrayService {
  public:
    virtual ~TrayService() = default;

    virtual ServiceStatus start(const TrayActions& actions) = 0;
    virtual void stop() = 0;
};

} // namespace Lexicon

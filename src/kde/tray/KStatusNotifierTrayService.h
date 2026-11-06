#pragma once

#include "core/interfaces/TrayService.h"

#include <KStatusNotifierItem>

#include <memory>

namespace Lexicon {

class KStatusNotifierTrayService final : public TrayService {
  public:
    ~KStatusNotifierTrayService() override;

    ServiceStatus start(const TrayActions& actions) override;
    void stop() override;

  private:
    std::unique_ptr<KStatusNotifierItem> m_item;
};

} // namespace Lexicon

#pragma once

#include "core/interfaces/SelectionProvider.h"
#include "core/selection/SelectionTextPolicy.h"

#include <memory>

namespace Lexicon {

/// Platform-neutral SelectionProvider that composes a PrimarySelectionSource
/// with SelectionTextPolicy.
///
/// Swapping Wayland for X11 or another desktop means supplying a different
/// source at the composition root; this class does not change.
class PrimarySelectionProvider final : public SelectionProvider {
  public:
    explicit PrimarySelectionProvider(std::unique_ptr<PrimarySelectionSource> source,
                                      SelectionTextPolicy policy = SelectionTextPolicy{});
    ~PrimarySelectionProvider() override;

    [[nodiscard]] SelectionResult currentSelection() override;
    [[nodiscard]] QString displayName() const override;

  private:
    std::unique_ptr<PrimarySelectionSource> m_source;
    SelectionTextPolicy m_policy;
};

} // namespace Lexicon

#pragma once

#include "core/interfaces/SelectionProvider.h"

namespace Lexicon {

class KdeSelectionProvider final : public SelectionProvider {
  public:
    [[nodiscard]] SelectionResult currentSelection() override;
    [[nodiscard]] QString displayName() const override;
};

} // namespace Lexicon

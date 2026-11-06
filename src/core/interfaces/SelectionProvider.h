#pragma once

#include "core/types/SelectionResult.h"

#include <QString>

namespace Lexicon {

class SelectionProvider {
  public:
    virtual ~SelectionProvider() = default;

    [[nodiscard]] virtual SelectionResult currentSelection() = 0;
    [[nodiscard]] virtual QString displayName() const = 0;
};

} // namespace Lexicon

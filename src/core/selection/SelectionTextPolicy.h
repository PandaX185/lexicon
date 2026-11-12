#pragma once

#include "core/interfaces/PrimarySelectionSource.h"
#include "core/types/SelectionResult.h"

#include <QStringList>

namespace Lexicon {

/// Application-level rules for turning raw primary-selection bytes into the
/// text passed to a dictionary lookup.
///
/// Kept free of platform code so the rules can be unit-tested without a
/// compositor and reused by any PrimarySelectionSource implementation.
class SelectionTextPolicy {
  public:
    static constexpr int DefaultMaxCharacters = 128;

    explicit SelectionTextPolicy(int maxCharacters = DefaultMaxCharacters);

    /// Accepted MIME types in preference order, lowercase.
    [[nodiscard]] const QStringList& acceptedMimeTypes() const noexcept {
        return m_mimeTypes;
    }

    [[nodiscard]] int maxCharacters() const noexcept {
        return m_maxCharacters;
    }

    [[nodiscard]] SelectionResult interpret(const PrimarySelectionReadResult& read) const;

  private:
    int m_maxCharacters;
    QStringList m_mimeTypes;
};

} // namespace Lexicon

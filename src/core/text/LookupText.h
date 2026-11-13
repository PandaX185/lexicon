#pragma once

#include <QString>

namespace Lexicon {

/// The maximum number of whitespace-separated words a dictionary lookup accepts.
/// Longer selections are sentences, not headwords, and are rejected before any
/// database work happens.
inline constexpr int MaxLookupWords = 4;

/// Cleans a raw lookup string without changing its meaning:
/// - trims and collapses whitespace,
/// - removes matching surrounding quotes or brackets,
/// - removes trailing sentence punctuation and unmatched closing quotes.
///
/// Internal punctuation (apostrophes, hyphens, dots in abbreviations) is kept.
[[nodiscard]] QString normalizeLookupText(const QString& raw);

/// Number of whitespace-separated words in already-normalized text.
[[nodiscard]] int countLookupWords(const QString& text);

} // namespace Lexicon

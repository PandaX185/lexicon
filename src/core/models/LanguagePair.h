#pragma once

#include <QString>

namespace Lexicon {

// A translation direction, e.g. English to German. The codes are lowercase
// BCP 47 tags, the names are the display names reported by the translation
// runtime and may be empty.
struct LanguagePair {
    QString source;
    QString target;
    QString sourceName;
    QString targetName;

    [[nodiscard]] bool isValid() const {
        return !source.isEmpty() && !target.isEmpty();
    }

    friend bool operator==(const LanguagePair& lhs, const LanguagePair& rhs) {
        return lhs.source == rhs.source && lhs.target == rhs.target;
    }
};

} // namespace Lexicon

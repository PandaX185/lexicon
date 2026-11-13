#pragma once

#include "config/Config.h"

#include <QString>
#include <QStringList>

namespace Lexicon {

// Paths used by the first-run fallback. Keeping them explicit makes the copy
// logic testable without touching the real XDG locations.
struct DictionarySeedPaths {
    QStringList populatedDirectories;
    QStringList sourceDirectories;
    QString destinationDirectory;
};

// Copies the bundled en.sqlite into the user dictionary directory when the
// configured locations do not contain a dictionary yet. Returns true only when
// a file was copied.
[[nodiscard]] bool seedUserDictionary(const DictionarySeedPaths& paths);

// Resolves the standard locations for the current user and seeds them.
[[nodiscard]] bool seedUserDictionary(const Config& config);

} // namespace Lexicon

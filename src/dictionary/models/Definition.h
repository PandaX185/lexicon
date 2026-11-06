#pragma once

#include "dictionary/models/PartOfSpeech.h"

#include <QString>

namespace Lexicon {

struct Definition {
    QString text;
    PartOfSpeech partOfSpeech = PartOfSpeech::Other;
    QString label;
};

} // namespace Lexicon

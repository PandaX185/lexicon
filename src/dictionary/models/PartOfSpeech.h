#pragma once

#include <QString>

namespace Lexicon {

enum class PartOfSpeech {
    Noun,
    Verb,
    Adjective,
    Adverb,
    Pronoun,
    Preposition,
    Conjunction,
    Determiner,
    Interjection,
    Other,
};

[[nodiscard]] QString partOfSpeechToString(PartOfSpeech partOfSpeech);
[[nodiscard]] PartOfSpeech partOfSpeechFromString(const QString& value);

} // namespace Lexicon

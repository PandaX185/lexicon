#include "dictionary/models/PartOfSpeech.h"

namespace Lexicon {

QString partOfSpeechToString(PartOfSpeech partOfSpeech) {
    switch (partOfSpeech) {
    case PartOfSpeech::Noun:
        return QStringLiteral("noun");
    case PartOfSpeech::Verb:
        return QStringLiteral("verb");
    case PartOfSpeech::Adjective:
        return QStringLiteral("adjective");
    case PartOfSpeech::Adverb:
        return QStringLiteral("adverb");
    case PartOfSpeech::Pronoun:
        return QStringLiteral("pronoun");
    case PartOfSpeech::Preposition:
        return QStringLiteral("preposition");
    case PartOfSpeech::Conjunction:
        return QStringLiteral("conjunction");
    case PartOfSpeech::Determiner:
        return QStringLiteral("determiner");
    case PartOfSpeech::Interjection:
        return QStringLiteral("interjection");
    case PartOfSpeech::Other:
        return QStringLiteral("other");
    }
    return QStringLiteral("other");
}

PartOfSpeech partOfSpeechFromString(const QString& value) {
    const QString normalized = value.trimmed().toLower();
    if (normalized == QStringLiteral("noun")) {
        return PartOfSpeech::Noun;
    }
    if (normalized == QStringLiteral("verb")) {
        return PartOfSpeech::Verb;
    }
    if (normalized == QStringLiteral("adjective")) {
        return PartOfSpeech::Adjective;
    }
    if (normalized == QStringLiteral("adverb")) {
        return PartOfSpeech::Adverb;
    }
    if (normalized == QStringLiteral("pronoun")) {
        return PartOfSpeech::Pronoun;
    }
    if (normalized == QStringLiteral("preposition")) {
        return PartOfSpeech::Preposition;
    }
    if (normalized == QStringLiteral("conjunction")) {
        return PartOfSpeech::Conjunction;
    }
    if (normalized == QStringLiteral("determiner")) {
        return PartOfSpeech::Determiner;
    }
    if (normalized == QStringLiteral("interjection")) {
        return PartOfSpeech::Interjection;
    }
    return PartOfSpeech::Other;
}

} // namespace Lexicon

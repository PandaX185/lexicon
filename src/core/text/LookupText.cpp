#include "core/text/LookupText.h"

#include <QChar>
#include <QStringList>

#include <array>
#include <utility>

namespace Lexicon {
namespace {

bool isSentencePunctuation(QChar character) {
    static const QString punctuation = QStringLiteral(",;:!?");
    return punctuation.contains(character);
}

bool isClosingCharacter(QChar character) {
    static const QString closing = QStringLiteral("\"'`\u201D\u2019\u00BB)]}");
    return closing.contains(character);
}

bool isOpeningCharacter(QChar character) {
    static const QString opening = QStringLiteral("\"'`\u201C\u2018\u00AB([{");
    return opening.contains(character);
}

struct CharacterWrapper {
    ushort opening;
    ushort closing;
};

constexpr std::array<CharacterWrapper, 8> Wrappers = {
    {
        {.opening = '"', .closing = '"'},
        {.opening = '\'', .closing = '\''},
        {.opening = 0x201C, .closing = 0x201D}, // “ ”
        {.opening = 0x2018, .closing = 0x2019}, // ‘ ’
        {.opening = '(', .closing = ')'},
        {.opening = '[', .closing = ']'},
        {.opening = '{', .closing = '}'},
        {.opening = 0x00AB, .closing = 0x00BB}, // « »
    },
};

// Removes a trailing run of dots only when it is sentence punctuation. One dot
// in a token with no other dots is a full stop ("word."); dots inside the token
// belong to an abbreviation ("U.S.").
void stripTrailingDots(QString& text) {
    if (!text.endsWith(QLatin1Char('.'))) {
        return;
    }

    const QString lastToken = text.section(QLatin1Char(' '), -1);
    QString withoutDots = lastToken;
    int dotCount = 0;
    while (withoutDots.endsWith(QLatin1Char('.'))) {
        withoutDots.chop(1);
        ++dotCount;
    }

    if (dotCount > 0 && !withoutDots.contains(QLatin1Char('.'))) {
        text.chop(dotCount);
    }
}

} // namespace

QString normalizeLookupText(const QString& raw) {
    QString text = raw.simplified();

    // Wrapping and punctuation can nest, for example `("word").`, so repeat a
    // bounded number of passes instead of trying to handle every combination.
    for (int pass = 0; pass < 4; ++pass) {
        bool changed = false;

        while (!text.isEmpty() && isSentencePunctuation(text.back())) {
            text.chop(1);
            changed = true;
        }

        // Strip nested wrappers until none remain (for example `("word")`).
        bool strippedWrapper = true;
        while (strippedWrapper && text.size() >= 2) {
            strippedWrapper = false;
            for (const auto& [opening, closing] : Wrappers) {
                if (text.front() == QChar(opening) && text.back() == QChar(closing)) {
                    text = text.mid(1, text.size() - 2).simplified();
                    changed = true;
                    strippedWrapper = true;
                    break;
                }
            }
        }

        while (!text.isEmpty() && isClosingCharacter(text.back())) {
            text.chop(1);
            changed = true;
        }

        // A leading apostrophe is kept unless it wraps the whole token, because
        // it is part of contractions such as 'tis.
        while (!text.isEmpty() && isOpeningCharacter(text.front())) {
            if (text.front() == QLatin1Char('\'') && text.size() > 1) {
                break;
            }
            text.remove(0, 1);
            changed = true;
        }

        const qsizetype beforeDots = text.size();
        stripTrailingDots(text);
        changed = changed || text.size() != beforeDots;

        text = text.simplified();
        if (!changed) {
            break;
        }
    }

    return text;
}

int countLookupWords(const QString& text) {
    return static_cast<int>(text.split(QLatin1Char(' '), Qt::SkipEmptyParts).size());
}

} // namespace Lexicon

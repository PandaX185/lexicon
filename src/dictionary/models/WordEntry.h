#pragma once

#include "dictionary/models/Definition.h"
#include "dictionary/models/Pronunciation.h"

#include <QHash>
#include <QList>
#include <QString>

namespace Lexicon {

struct WordEntry {
    QString word;
    QList<Definition> definitions;
    Pronunciation pronunciation;
    QHash<QString, QString> metadata;

    [[nodiscard]] bool isValid() const {
        return !word.trimmed().isEmpty() && !definitions.isEmpty();
    }
    [[nodiscard]] QString normalizedWord() const {
        return word.trimmed().toCaseFolded();
    }
};

} // namespace Lexicon

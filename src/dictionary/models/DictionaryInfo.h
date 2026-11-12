#pragma once

#include <QString>

namespace Lexicon {

// Language tags are lowercase BCP 47 tags ("en", "de", "pt-br"). Lexicon never
// guesses which language a selected word is in: the pair always comes from the
// dictionary's own metadata and the configured preference.
[[nodiscard]] inline QString normalizeLanguageTag(const QString& tag) {
    return tag.trimmed().toLower().replace(QLatin1Char('_'), QLatin1Char('-'));
}

// Describes one dictionary database: `sourceLanguage` is the language of the
// headwords, `targetLanguage` the language of the definitions.
struct DictionaryInfo {
    QString name;
    QString sourceLanguage;
    QString targetLanguage;
    QString license;
    QString attribution;

    [[nodiscard]] bool isValid() const {
        return !name.trimmed().isEmpty() && !sourceLanguage.isEmpty() && !targetLanguage.isEmpty();
    }

    [[nodiscard]] bool isMonolingual() const {
        return sourceLanguage == targetLanguage;
    }

    [[nodiscard]] bool matches(const QString& source, const QString& target) const {
        return sourceLanguage == normalizeLanguageTag(source) && targetLanguage == normalizeLanguageTag(target);
    }

    // "en" for a monolingual dictionary, "en -> de" for a translation dictionary.
    [[nodiscard]] QString languagePairLabel() const {
        if (isMonolingual() || targetLanguage.isEmpty()) {
            return sourceLanguage;
        }
        return sourceLanguage + QStringLiteral(" \u2192 ") + targetLanguage;
    }
};

} // namespace Lexicon

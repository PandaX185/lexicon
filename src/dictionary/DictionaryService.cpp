#include "dictionary/DictionaryService.h"

#include "core/text/LookupText.h"

#include <algorithm>
#include <utility>

namespace Lexicon {

DictionaryService::DictionaryService(std::unique_ptr<DictionaryRepository> repository, int cacheSize)
    : m_repository(std::move(repository)) {
    cacheSize = std::max(cacheSize, 0);
    m_cache.setMaxCost(cacheSize);
    m_cacheEnabled = cacheSize > 0;
}

DictionaryLookupResult DictionaryService::lookup(const QString& rawWord) const {
    const QString word = normalizeLookupText(rawWord);
    if (word.isEmpty()) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::InvalidArgument, QStringLiteral("Enter a word to look up.")));
    }

    const int wordCount = countLookupWords(word);
    if (wordCount > MaxLookupWords) {
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::InvalidArgument,
            QStringLiteral("Select a single word or a short phrase of at most %1 words.").arg(MaxLookupWords),
            QStringLiteral("The selection contains %1 words.").arg(wordCount)));
    }

    if (!m_repository) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary repository is configured.")));
    }

    if (m_cacheEnabled) {
        auto* const cached = m_cache.object(word);
        if (cached) {
            return *cached;
        }
    }

    DictionaryLookupResult result = m_repository->findByWord(word);
    if (result.error.isError()) {
        return result;
    }
    if (!result.found()) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::WordNotFound, QStringLiteral("No entry found for \"%1\".").arg(word), word));
    }

    if (m_cacheEnabled && m_cache.maxCost() > 0) {
        m_cache.insert(word, new DictionaryLookupResult(result));
    }
    return result;
}

QString DictionaryService::repositoryLocation() const {
    return m_repository ? m_repository->location() : QString();
}

void DictionaryService::clearCache() const {
    m_cache.clear();
}

} // namespace Lexicon

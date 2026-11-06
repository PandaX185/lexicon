#include "dictionary/DictionaryService.h"

#include <utility>

namespace Lexicon {

DictionaryService::DictionaryService(std::unique_ptr<DictionaryRepository> repository)
    : m_repository(std::move(repository)) {}

DictionaryLookupResult DictionaryService::lookup(const QString& rawWord) const {
    const QString word = rawWord.trimmed();
    if (word.isEmpty()) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::InvalidArgument, QStringLiteral("Enter a word to look up.")));
    }

    if (!m_repository) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary repository is configured.")));
    }

    DictionaryLookupResult result = m_repository->findByWord(word);
    if (result.error.isError()) {
        return result;
    }
    if (!result.found()) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::WordNotFound, QStringLiteral("No entry found for \"%1\".").arg(word), word));
    }
    return result;
}

QString DictionaryService::repositoryLocation() const {
    return m_repository ? m_repository->location() : QString();
}

} // namespace Lexicon

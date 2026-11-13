#pragma once

#include "dictionary/DictionaryRepository.h"

#include <QCache>
#include <QString>

#include <memory>

namespace Lexicon {

class DictionaryService {
  public:
    explicit DictionaryService(std::unique_ptr<DictionaryRepository> repository, int cacheSize = 256);

    [[nodiscard]] DictionaryLookupResult lookup(const QString& rawWord) const;
    [[nodiscard]] QString repositoryLocation() const;

    // Clears any cached lookups. Call after the underlying dictionary changes.
    void clearCache() const;

  private:
    std::unique_ptr<DictionaryRepository> m_repository;
    mutable QCache<QString, DictionaryLookupResult> m_cache;
    mutable bool m_cacheEnabled = true;
};

} // namespace Lexicon

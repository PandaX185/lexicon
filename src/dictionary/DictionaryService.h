#pragma once

#include "dictionary/DictionaryRepository.h"

#include <QString>

#include <memory>

namespace Lexicon {

class DictionaryService {
  public:
    explicit DictionaryService(std::unique_ptr<DictionaryRepository> repository);

    [[nodiscard]] DictionaryLookupResult lookup(const QString& rawWord) const;
    [[nodiscard]] QString repositoryLocation() const;

  private:
    std::unique_ptr<DictionaryRepository> m_repository;
};

} // namespace Lexicon

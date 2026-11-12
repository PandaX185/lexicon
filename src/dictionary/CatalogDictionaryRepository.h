#pragma once

#include "dictionary/DictionaryCatalog.h"
#include "dictionary/DictionaryRepository.h"

#include <QString>

#include <optional>

namespace Lexicon {

// Resolves the configured language pair against the installed dictionaries and
// delegates the lookup to whichever database covers it. This keeps language
// support a matter of installed content instead of hard-coded behaviour.
class CatalogDictionaryRepository final : public DictionaryRepository {
  public:
    CatalogDictionaryRepository(DictionaryCatalog catalog, DictionaryCatalog::RepositoryFactory factory,
                                QString sourceLanguage, QString targetLanguage);

    [[nodiscard]] DictionaryLookupResult findByWord(const QString& word) const override;
    [[nodiscard]] DictionaryDescription describe() const override;
    [[nodiscard]] QString location() const override;

  private:
    [[nodiscard]] std::optional<DictionaryCatalogEntry> resolve() const;

    DictionaryCatalog m_catalog;
    DictionaryCatalog::RepositoryFactory m_factory;
    QString m_sourceLanguage;
    QString m_targetLanguage;
};

} // namespace Lexicon

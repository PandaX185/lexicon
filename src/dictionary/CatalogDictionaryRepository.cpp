#include "dictionary/CatalogDictionaryRepository.h"

#include <utility>

namespace Lexicon {

namespace {

AppError missingPairError(const QString& sourceLanguage, const QString& targetLanguage, const QString& directory) {
    const DictionaryInfo requested{
        .name = QStringLiteral("requested"),
        .sourceLanguage = normalizeLanguageTag(sourceLanguage),
        .targetLanguage = normalizeLanguageTag(targetLanguage),
        .license = {},
        .attribution = {},
    };
    return AppError::make(ErrorCode::DatabaseUnavailable,
                          QStringLiteral("No dictionary installed for %1.").arg(requested.languagePairLabel()),
                          directory);
}

} // namespace

CatalogDictionaryRepository::CatalogDictionaryRepository(DictionaryCatalog catalog,
                                                         DictionaryCatalog::RepositoryFactory factory,
                                                         QString sourceLanguage, QString targetLanguage)
    : m_catalog(std::move(catalog)), m_factory(std::move(factory)), m_sourceLanguage(std::move(sourceLanguage)),
      m_targetLanguage(std::move(targetLanguage)) {}

std::optional<DictionaryCatalogEntry> CatalogDictionaryRepository::resolve() const {
    return m_catalog.find(m_sourceLanguage, m_targetLanguage);
}

DictionaryLookupResult CatalogDictionaryRepository::findByWord(const QString& word) const {
    const std::optional<DictionaryCatalogEntry> entry = resolve();
    if (!entry) {
        return DictionaryLookupResult::failure(
            missingPairError(m_sourceLanguage, m_targetLanguage, m_catalog.directory()));
    }
    if (!m_factory) {
        return DictionaryLookupResult::failure(
            AppError::make(ErrorCode::DatabaseUnavailable, QStringLiteral("No dictionary repository is configured.")));
    }

    const std::unique_ptr<DictionaryRepository> repository = m_factory(entry->path);
    if (!repository) {
        return DictionaryLookupResult::failure(AppError::make(
            ErrorCode::DatabaseUnavailable, QStringLiteral("Could not open the installed dictionary."), entry->path));
    }
    return repository->findByWord(word);
}

DictionaryDescription CatalogDictionaryRepository::describe() const {
    const std::optional<DictionaryCatalogEntry> entry = resolve();
    if (!entry) {
        return DictionaryDescription::failure(
            missingPairError(m_sourceLanguage, m_targetLanguage, m_catalog.directory()));
    }

    DictionaryDescription description;
    description.info = entry->info;
    description.entryCount = entry->entryCount;
    return description;
}

QString CatalogDictionaryRepository::location() const {
    const std::optional<DictionaryCatalogEntry> entry = resolve();
    return entry ? entry->path : m_catalog.directory();
}

} // namespace Lexicon

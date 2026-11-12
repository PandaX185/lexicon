#pragma once

#include "core/errors/AppError.h"
#include "dictionary/models/DictionaryInfo.h"
#include "dictionary/models/WordEntry.h"

#include <QString>

#include <optional>
#include <utility>

namespace Lexicon {

struct DictionaryLookupResult {
    std::optional<WordEntry> entry;
    AppError error;

    [[nodiscard]] bool found() const noexcept {
        return entry.has_value();
    }

    static DictionaryLookupResult success(WordEntry value) {
        DictionaryLookupResult result;
        result.entry = std::move(value);
        return result;
    }

    static DictionaryLookupResult notFound() {
        return {};
    }

    static DictionaryLookupResult failure(AppError appError) {
        DictionaryLookupResult result;
        result.error = std::move(appError);
        return result;
    }
};

// What a repository reports about the dictionary behind it, so the catalog can
// decide whether the file covers the requested language pair.
struct DictionaryDescription {
    DictionaryInfo info;
    qint64 entryCount = 0;
    AppError error;

    [[nodiscard]] bool ok() const noexcept {
        return !error.isError();
    }

    static DictionaryDescription failure(AppError appError) {
        DictionaryDescription description;
        description.error = std::move(appError);
        return description;
    }
};

class DictionaryRepository {
  public:
    virtual ~DictionaryRepository() = default;

    [[nodiscard]] virtual DictionaryLookupResult findByWord(const QString& word) const = 0;
    [[nodiscard]] virtual DictionaryDescription describe() const = 0;
    [[nodiscard]] virtual QString location() const = 0;
};

} // namespace Lexicon

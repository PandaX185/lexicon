#pragma once

#include "core/errors/AppError.h"
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

class DictionaryRepository {
  public:
    virtual ~DictionaryRepository() = default;

    [[nodiscard]] virtual DictionaryLookupResult findByWord(const QString& word) const = 0;
    [[nodiscard]] virtual QString location() const = 0;
};

} // namespace Lexicon

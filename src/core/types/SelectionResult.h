#pragma once

#include "core/errors/AppError.h"

#include <QString>

#include <optional>
#include <utility>

namespace Lexicon {

struct SelectionResult {
    std::optional<QString> text;
    AppError error;

    [[nodiscard]] bool hasSelection() const noexcept {
        return text.has_value() && !text->trimmed().isEmpty();
    }

    static SelectionResult success(QString value) {
        SelectionResult result;
        result.text = std::move(value);
        return result;
    }

    static SelectionResult failure(AppError appError) {
        SelectionResult result;
        result.error = std::move(appError);
        return result;
    }
};

} // namespace Lexicon

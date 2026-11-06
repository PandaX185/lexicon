#pragma once

#include "core/errors/AppError.h"

#include <utility>

namespace Lexicon {

struct ServiceStatus {
    bool available = false;
    AppError error;

    static ServiceStatus success() {
        return ServiceStatus{.available = true, .error = AppError::none()};
    }

    static ServiceStatus failure(AppError appError) {
        return ServiceStatus{.available = false, .error = std::move(appError)};
    }
};

} // namespace Lexicon

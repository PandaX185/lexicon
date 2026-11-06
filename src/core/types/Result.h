#pragma once

#include "core/errors/AppError.h"

#include <QtGlobal>

#include <optional>
#include <utility>

namespace Lexicon {

template <typename T> class Result {
  public:
    Result(T value) : m_value(std::move(value)) {}

    Result(AppError error) : m_error(std::move(error)) {}

    [[nodiscard]] bool hasValue() const noexcept {
        return m_value.has_value();
    }
    explicit operator bool() const noexcept {
        return hasValue();
    }

    [[nodiscard]] const T& value() const {
        Q_ASSERT(m_value.has_value());
        return *m_value;
    }

    [[nodiscard]] T& value() {
        Q_ASSERT(m_value.has_value());
        return *m_value;
    }

    [[nodiscard]] const T* operator->() const {
        return &value();
    }
    [[nodiscard]] T* operator->() {
        return &value();
    }

    [[nodiscard]] const AppError& error() const noexcept {
        return m_error;
    }

  private:
    std::optional<T> m_value;
    AppError m_error;
};

} // namespace Lexicon

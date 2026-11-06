#pragma once

#include <QMetaType>
#include <QString>

namespace Lexicon {

enum class ErrorCode {
    None,
    InvalidArgument,
    ConfigParseError,
    ConfigValidationError,
    DatabaseUnavailable,
    DatabaseOpenFailed,
    DatabaseQueryFailed,
    WordNotFound,
    SelectionUnavailable,
    ShortcutRegistrationFailed,
    TrayUnavailable,
    Unknown,
};

struct AppError {
    ErrorCode code = ErrorCode::None;
    QString message;
    QString detail;

    [[nodiscard]] bool isError() const noexcept {
        return code != ErrorCode::None;
    }
    explicit operator bool() const noexcept {
        return isError();
    }

    static AppError none();
    static AppError make(ErrorCode code, QString message, QString detail = {});
};

[[nodiscard]] QString errorCodeToString(ErrorCode code);

} // namespace Lexicon

Q_DECLARE_METATYPE(Lexicon::AppError)

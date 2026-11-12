#include "core/errors/AppError.h"

#include <utility>

namespace Lexicon {

AppError AppError::none() {
    return {};
}

AppError AppError::make(ErrorCode code, QString message, QString detail) {
    return AppError{.code = code, .message = std::move(message), .detail = std::move(detail)};
}

QString errorCodeToString(ErrorCode code) {
    switch (code) {
    case ErrorCode::None:
        return QStringLiteral("none");
    case ErrorCode::InvalidArgument:
        return QStringLiteral("invalid_argument");
    case ErrorCode::ConfigParseError:
        return QStringLiteral("config_parse_error");
    case ErrorCode::ConfigValidationError:
        return QStringLiteral("config_validation_error");
    case ErrorCode::DatabaseUnavailable:
        return QStringLiteral("database_unavailable");
    case ErrorCode::DatabaseOpenFailed:
        return QStringLiteral("database_open_failed");
    case ErrorCode::DatabaseQueryFailed:
        return QStringLiteral("database_query_failed");
    case ErrorCode::WordNotFound:
        return QStringLiteral("word_not_found");
    case ErrorCode::SelectionUnavailable:
        return QStringLiteral("selection_unavailable");
    case ErrorCode::SelectionUnsupported:
        return QStringLiteral("selection_unsupported");
    case ErrorCode::SelectionNotText:
        return QStringLiteral("selection_not_text");
    case ErrorCode::SelectionTooLong:
        return QStringLiteral("selection_too_long");
    case ErrorCode::TranslationUnavailable:
        return QStringLiteral("translation_unavailable");
    case ErrorCode::ProtocolViolation:
        return QStringLiteral("protocol_violation");
    case ErrorCode::ShortcutRegistrationFailed:
        return QStringLiteral("shortcut_registration_failed");
    case ErrorCode::TrayUnavailable:
        return QStringLiteral("tray_unavailable");
    case ErrorCode::Unknown:
        return QStringLiteral("unknown");
    }
    return QStringLiteral("unknown");
}

} // namespace Lexicon

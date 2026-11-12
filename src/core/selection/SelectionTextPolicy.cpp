#include "core/selection/SelectionTextPolicy.h"

#include <QStringDecoder>

#include <algorithm>
#include <utility>

namespace Lexicon {

SelectionTextPolicy::SelectionTextPolicy(int maxCharacters)
    : m_maxCharacters(std::max(1, maxCharacters)), m_mimeTypes{
                                                       QStringLiteral("text/plain;charset=utf-8"),
                                                       QStringLiteral("text/plain"),
                                                       QStringLiteral("utf8_string"),
                                                   } {}

SelectionResult SelectionTextPolicy::interpret(const PrimarySelectionReadResult& read) const {
    if (read.error.isError()) {
        return SelectionResult::failure(read.error);
    }

    if (!read.selection) {
        return SelectionResult::failure(
            AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("No text is currently selected.")));
    }

    const RawSelection& raw = *read.selection;

    const bool supportedMimeType = std::ranges::any_of(m_mimeTypes, [&raw](const QString& accepted) {
        return accepted.compare(raw.mimeType, Qt::CaseInsensitive) == 0;
    });
    if (!supportedMimeType) {
        return SelectionResult::failure(AppError::make(
            ErrorCode::SelectionNotText, QStringLiteral("The current selection does not contain plain text."),
            QStringLiteral("Offered type: %1").arg(raw.mimeType)));
    }

    QStringDecoder decoder(QStringConverter::Utf8, QStringConverter::Flag::Stateless);
    QString text = decoder(raw.data);
    if (decoder.hasError()) {
        return SelectionResult::failure(AppError::make(
            ErrorCode::SelectionNotText, QStringLiteral("The current selection is not valid UTF-8 text.")));
    }

    text = text.trimmed();
    if (text.isEmpty()) {
        return SelectionResult::failure(
            AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("No text is currently selected.")));
    }

    if (text.size() > m_maxCharacters) {
        return SelectionResult::failure(AppError::make(
            ErrorCode::SelectionTooLong,
            QStringLiteral("The selection is too long to look up (maximum %1 characters).").arg(m_maxCharacters),
            QStringLiteral("Selected %1 characters.").arg(text.size())));
    }

    return SelectionResult::success(std::move(text));
}

} // namespace Lexicon

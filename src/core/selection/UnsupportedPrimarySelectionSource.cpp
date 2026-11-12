#include "core/selection/UnsupportedPrimarySelectionSource.h"

#include <utility>

namespace Lexicon {

UnsupportedPrimarySelectionSource::UnsupportedPrimarySelectionSource(QString displayName, QString message,
                                                                     QString detail)
    : m_displayName(std::move(displayName)), m_message(std::move(message)), m_detail(std::move(detail)) {}

PrimarySelectionReadResult
UnsupportedPrimarySelectionSource::readPrimarySelection([[maybe_unused]] const QStringList& mimeTypes) {
    return PrimarySelectionReadResult::failure(AppError::make(ErrorCode::SelectionUnsupported, m_message, m_detail));
}

QString UnsupportedPrimarySelectionSource::displayName() const {
    return m_displayName;
}

} // namespace Lexicon

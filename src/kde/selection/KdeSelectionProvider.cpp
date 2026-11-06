#include "kde/selection/KdeSelectionProvider.h"

namespace Lexicon {

SelectionResult KdeSelectionProvider::currentSelection() {
    return SelectionResult::failure(AppError::make(
        ErrorCode::SelectionUnavailable, QStringLiteral("Selection retrieval is not implemented in this scaffold yet."),
        QStringLiteral("Wayland primary-selection support is the Stage 2 proof of concept. "
                       "See docs/architecture/selection-retrieval.md.")));
}

QString KdeSelectionProvider::displayName() const {
    return QStringLiteral("KDE/Wayland primary selection (proof of concept pending)");
}

} // namespace Lexicon

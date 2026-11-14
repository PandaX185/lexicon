#include "app/PopupPlacement.h"

#include <algorithm>

namespace Lexicon {

std::optional<QPoint> placePopup(const QRect& available, const QPoint& cursor, const QSize& popupSize,
                                 PopupPosition mode) {
    if (popupSize.width() > available.width() || popupSize.height() > available.height()) {
        return std::nullopt;
    }

    QPoint position;
    switch (mode) {
    case PopupPosition::Center:
        position = available.center() - QPoint(popupSize.width() / 2, popupSize.height() / 2);
        break;
    case PopupPosition::Cursor:
    case PopupPosition::NearSelection:
        position = cursor + QPoint(PopupCursorOffset, PopupCursorOffset);
        break;
    }

    const int maxX = std::max(available.left(), available.right() - popupSize.width());
    const int maxY = std::max(available.top(), available.bottom() - popupSize.height());

    position.setX(std::clamp(position.x(), available.left(), maxX));
    position.setY(std::clamp(position.y(), available.top(), maxY));
    return position;
}

} // namespace Lexicon

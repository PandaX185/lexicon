#pragma once

#include "config/Config.h"

#include <QPoint>
#include <QRect>
#include <QSize>

#include <optional>

namespace Lexicon {

/// Distance between the cursor and the popup for cursor-relative placement.
inline constexpr int PopupCursorOffset = 16;

/// Compute a popup position inside `available` (logical screen coordinates).
///
/// Returns no value when the popup cannot fit on the screen at all. The caller
/// keeps the compositor-chosen position in that case and logs the mismatch.
[[nodiscard]] std::optional<QPoint> placePopup(const QRect& available, const QPoint& cursor, const QSize& popupSize,
                                               PopupPosition mode);

} // namespace Lexicon

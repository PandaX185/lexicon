#pragma once

#include "core/errors/AppError.h"

#include <QByteArray>
#include <QString>
#include <QStringList>

#include <optional>
#include <utility>

namespace Lexicon {

/// Raw bytes read from a platform selection, before any application-level
/// interpretation (UTF-8 decoding, trimming, length checks).
struct RawSelection {
    QString mimeType;
    QByteArray data;
};

/// Result of asking a platform source for the current primary selection.
///
/// The source reports transport-level facts only: unsupported protocol, no
/// selection, no acceptable MIME type, or the raw bytes. Policy decisions
/// belong to SelectionTextPolicy so they can be tested without a compositor.
struct PrimarySelectionReadResult {
    std::optional<RawSelection> selection;
    AppError error;

    [[nodiscard]] bool hasSelection() const noexcept {
        return selection.has_value();
    }

    static PrimarySelectionReadResult success(QString mimeType, QByteArray data) {
        PrimarySelectionReadResult result;
        result.selection = RawSelection{.mimeType = std::move(mimeType), .data = std::move(data)};
        return result;
    }

    static PrimarySelectionReadResult failure(AppError error) {
        PrimarySelectionReadResult result;
        result.error = std::move(error);
        return result;
    }
};

/// Platform-specific way to read the primary selection.
///
/// Implementations exist per platform/protocol (currently Wayland
/// ext-data-control-v1; X11 can implement the same interface later). The rest
/// of the application depends only on this interface and on SelectionProvider,
/// never on a specific display server or toolkit.
class PrimarySelectionSource {
  public:
    virtual ~PrimarySelectionSource() = default;

    /// Read the current primary selection, accepting the given MIME types in
    /// preference order. Implementations must not log the returned bytes.
    [[nodiscard]] virtual PrimarySelectionReadResult readPrimarySelection(const QStringList& mimeTypes) = 0;

    [[nodiscard]] virtual QString displayName() const = 0;
};

} // namespace Lexicon

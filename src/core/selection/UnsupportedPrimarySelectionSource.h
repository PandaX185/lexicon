#pragma once

#include "core/interfaces/PrimarySelectionSource.h"

namespace Lexicon {

/// Explicit "this platform is not wired up yet" source.
///
/// Used by the composition root for sessions without selection support (for
/// example X11 until an X11 source exists) so the UI can report a precise
/// error instead of silently reading the clipboard.
class UnsupportedPrimarySelectionSource final : public PrimarySelectionSource {
  public:
    UnsupportedPrimarySelectionSource(QString displayName, QString message, QString detail = {});

    [[nodiscard]] PrimarySelectionReadResult readPrimarySelection(const QStringList& mimeTypes) override;
    [[nodiscard]] QString displayName() const override;

  private:
    QString m_displayName;
    QString m_message;
    QString m_detail;
};

} // namespace Lexicon

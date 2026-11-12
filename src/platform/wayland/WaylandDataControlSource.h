#pragma once

#include "core/interfaces/PrimarySelectionSource.h"

#include <chrono>

namespace Lexicon {

/// Reads the primary selection through the Wayland ext-data-control-v1
/// protocol.
///
/// Each call opens a short-lived connection, requests the current primary
/// selection, and reads it with a bounded timeout. It never falls back to the
/// clipboard and never logs the selected bytes.
class WaylandDataControlSource final : public PrimarySelectionSource {
  public:
    explicit WaylandDataControlSource(std::chrono::milliseconds ioTimeout = std::chrono::milliseconds(500));

    [[nodiscard]] PrimarySelectionReadResult readPrimarySelection(const QStringList& mimeTypes) override;
    [[nodiscard]] QString displayName() const override;

  private:
    std::chrono::milliseconds m_ioTimeout;
};

} // namespace Lexicon

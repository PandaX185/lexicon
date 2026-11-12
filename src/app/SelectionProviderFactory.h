#pragma once

#include <memory>

namespace Lexicon {

class SelectionProvider;

/// Composition-root factory: chooses the platform source for the current
/// session and wraps it in the platform-neutral provider.
///
/// This is the only place that knows about concrete platforms. Adding X11 or
/// another desktop means adding a source and a branch here; core, the
/// controller, and the UI stay unchanged.
[[nodiscard]] std::unique_ptr<SelectionProvider> createSelectionProvider();

} // namespace Lexicon

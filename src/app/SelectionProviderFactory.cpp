#include "app/SelectionProviderFactory.h"

#include "core/selection/PrimarySelectionProvider.h"
#include "core/selection/UnsupportedPrimarySelectionSource.h"

#if defined(LEXICON_HAVE_WAYLAND)
#include "platform/wayland/WaylandDataControlSource.h"
#endif

#include <QString>
#include <QtGlobal>

#include <memory>

namespace Lexicon {
namespace {

enum class DisplayServer {
    Wayland,
    X11,
    Other,
};

DisplayServer detectDisplayServer() {
    const QString sessionType = qEnvironmentVariable("XDG_SESSION_TYPE").trimmed().toLower();
    if (sessionType == QStringLiteral("wayland")) {
        return DisplayServer::Wayland;
    }
    if (sessionType == QStringLiteral("x11")) {
        return DisplayServer::X11;
    }
    if (!qEnvironmentVariableIsEmpty("WAYLAND_DISPLAY")) {
        return DisplayServer::Wayland;
    }
    if (!qEnvironmentVariableIsEmpty("DISPLAY")) {
        return DisplayServer::X11;
    }
    return DisplayServer::Other;
}

std::unique_ptr<SelectionProvider> unsupportedProvider(const QString& displayName, const QString& message,
                                                       const QString& detail = {}) {
    return std::make_unique<PrimarySelectionProvider>(
        std::make_unique<UnsupportedPrimarySelectionSource>(displayName, message, detail));
}

std::unique_ptr<SelectionProvider> waylandProvider() {
#if defined(LEXICON_HAVE_WAYLAND)
    return std::make_unique<PrimarySelectionProvider>(std::make_unique<WaylandDataControlSource>());
#else
    return unsupportedProvider(
        QStringLiteral("Wayland primary selection (backend not built)"),
        QStringLiteral("This build does not include the Wayland selection backend."),
        QStringLiteral("Rebuild with -DLEXICON_WITH_WAYLAND=ON and the wayland-client development files."));
#endif
}

std::unique_ptr<SelectionProvider> x11Provider() {
    // Decision (Stage 2): X11 is not implemented yet. It reports an explicit
    // unsupported error instead of falling back to the clipboard. A future
    // X11PrimarySelectionSource only needs to implement PrimarySelectionSource
    // and be selected here; nothing above this factory changes.
    return unsupportedProvider(
        QStringLiteral("X11 primary selection (not implemented)"),
        QStringLiteral("Automatic selection retrieval is not available on X11 yet."),
        QStringLiteral("Select text manually or use a Wayland session; the clipboard is not used as a fallback."));
}

} // namespace

std::unique_ptr<SelectionProvider> createSelectionProvider() {
    switch (detectDisplayServer()) {
    case DisplayServer::Wayland:
        return waylandProvider();
    case DisplayServer::X11:
        return x11Provider();
    case DisplayServer::Other:
        break;
    }

    return unsupportedProvider(
        QStringLiteral("No display server"),
        QStringLiteral("Automatic selection retrieval is unavailable without a display server."));
}

} // namespace Lexicon

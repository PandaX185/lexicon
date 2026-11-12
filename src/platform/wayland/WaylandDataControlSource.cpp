#include "platform/wayland/WaylandDataControlSource.h"

#include "ext-data-control-v1-client-protocol.h"

#include <QHash>
#include <QScopeGuard>
#include <QStringList>

#include <algorithm>
#include <array>
#include <cerrno>
#include <chrono>
#include <cstdint>
#include <cstring>
#include <fcntl.h>
#include <memory>
#include <optional>
#include <poll.h>
#include <unistd.h>
#include <utility>
#include <wayland-client.h>

namespace Lexicon {
namespace {

constexpr std::uint32_t ManagerVersion = 1;
constexpr std::uint32_t SeatVersion = 1;

// Any accepted text that fits in this buffer is far below the policy's
// character limit. Reading stops here so a hostile or broken source cannot
// make us allocate without bound.
constexpr int MaxReceivedBytes = 4096;

struct DisplayDeleter {
    void operator()(wl_display* display) const noexcept {
        if (display) {
            wl_display_disconnect(display);
        }
    }
};

using DisplayPtr = std::unique_ptr<wl_display, DisplayDeleter>;

struct OfferInfo {
    QStringList mimeTypes;
};

struct ReadContext {
    wl_registry* registry = nullptr;
    wl_seat* seat = nullptr;
    ext_data_control_manager_v1* manager = nullptr;
    ext_data_control_device_v1* device = nullptr;
    QHash<ext_data_control_offer_v1*, OfferInfo> offers;
    ext_data_control_offer_v1* primaryOffer = nullptr;
    bool primarySelectionReceived = false;

    ReadContext() = default;
    ReadContext(const ReadContext&) = delete;
    ReadContext& operator=(const ReadContext&) = delete;

    ~ReadContext() {
        for (auto it = offers.cbegin(); it != offers.cend(); ++it) {
            ext_data_control_offer_v1_destroy(it.key());
        }
        if (device) {
            ext_data_control_device_v1_destroy(device);
        }
        if (manager) {
            ext_data_control_manager_v1_destroy(manager);
        }
        if (seat) {
            wl_seat_destroy(seat);
        }
        if (registry) {
            wl_registry_destroy(registry);
        }
    }
};

PrimarySelectionReadResult failure(ErrorCode code, const QString& message, const QString& detail = {}) {
    return PrimarySelectionReadResult::failure(AppError::make(code, message, detail));
}

QString systemErrorMessage(int errorNumber) {
    return QString::fromLocal8Bit(std::strerror(errorNumber));
}

std::chrono::milliseconds remainingTime(std::chrono::steady_clock::time_point deadline) {
    const auto remaining =
        std::chrono::duration_cast<std::chrono::milliseconds>(deadline - std::chrono::steady_clock::now());
    return std::max(remaining, std::chrono::milliseconds(0));
}

int pollTimeout(std::chrono::milliseconds timeout) {
    return static_cast<int>(std::clamp<std::int64_t>(timeout.count(), 0, 60'000));
}

bool flushDisplay(wl_display* display, std::chrono::milliseconds timeout) {
    const auto deadline = std::chrono::steady_clock::now() + timeout;
    while (wl_display_flush(display) < 0) {
        if (errno != EAGAIN) {
            return false;
        }
        const auto remaining = remainingTime(deadline);
        if (remaining.count() == 0) {
            return false;
        }
        pollfd descriptor{.fd = wl_display_get_fd(display), .events = POLLOUT, .revents = 0};
        const int result = ::poll(&descriptor, 1, pollTimeout(remaining));
        if (result < 0 && errno != EINTR) {
            return false;
        }
        if (result == 0) {
            return false;
        }
    }
    return true;
}

bool waitForPrimarySelection(wl_display* display, ReadContext& context, std::chrono::milliseconds timeout) {
    const auto deadline = std::chrono::steady_clock::now() + timeout;
    while (!context.primarySelectionReceived) {
        const auto remaining = remainingTime(deadline);
        if (remaining.count() == 0) {
            return false;
        }
        pollfd descriptor{.fd = wl_display_get_fd(display), .events = POLLIN, .revents = 0};
        const int result = ::poll(&descriptor, 1, pollTimeout(remaining));
        if (result < 0) {
            if (errno == EINTR) {
                continue;
            }
            return false;
        }
        if (result == 0) {
            return false;
        }
        if (wl_display_dispatch(display) < 0) {
            return false;
        }
    }
    return true;
}

std::optional<QString> chooseMimeType(const QStringList& accepted, const QStringList& offered) {
    for (const QString& acceptedType : accepted) {
        for (const QString& offeredType : offered) {
            if (acceptedType.compare(offeredType, Qt::CaseInsensitive) == 0) {
                return offeredType;
            }
        }
    }
    return std::nullopt;
}

void registryGlobal(void* data, wl_registry* registry, std::uint32_t name, const char* interface,
                    std::uint32_t version) {
    auto& context = *static_cast<ReadContext*>(data);

    if (std::strcmp(interface, ext_data_control_manager_v1_interface.name) == 0 && !context.manager) {
        context.manager = static_cast<ext_data_control_manager_v1*>(wl_registry_bind(
            registry, name, &ext_data_control_manager_v1_interface, std::min(version, ManagerVersion)));
    } else if (std::strcmp(interface, wl_seat_interface.name) == 0 && !context.seat) {
        context.seat =
            static_cast<wl_seat*>(wl_registry_bind(registry, name, &wl_seat_interface, std::min(version, SeatVersion)));
    }
}

void registryGlobalRemove([[maybe_unused]] void* data, [[maybe_unused]] wl_registry* registry,
                          [[maybe_unused]] std::uint32_t name) {}

const wl_registry_listener RegistryListener = {
    .global = registryGlobal,
    .global_remove = registryGlobalRemove,
};

void offerMimeType(void* data, ext_data_control_offer_v1* offer, const char* mimeType) {
    auto& context = *static_cast<ReadContext*>(data);
    context.offers[offer].mimeTypes.append(QString::fromLatin1(mimeType));
}

const ext_data_control_offer_v1_listener OfferListener = {
    .offer = offerMimeType,
};

void deviceDataOffer(void* data, [[maybe_unused]] ext_data_control_device_v1* device,
                     ext_data_control_offer_v1* offer) {
    if (!offer) {
        return;
    }
    auto& context = *static_cast<ReadContext*>(data);
    context.offers.insert(offer, OfferInfo{});
    ext_data_control_offer_v1_add_listener(offer, &OfferListener, &context);
}

void deviceSelection([[maybe_unused]] void* data, [[maybe_unused]] ext_data_control_device_v1* device,
                     [[maybe_unused]] ext_data_control_offer_v1* offer) {
    // Only the primary selection is relevant for selected-text lookup.
}

void deviceFinished([[maybe_unused]] void* data, [[maybe_unused]] ext_data_control_device_v1* device) {}

void devicePrimarySelection(void* data, [[maybe_unused]] ext_data_control_device_v1* device,
                            ext_data_control_offer_v1* offer) {
    auto& context = *static_cast<ReadContext*>(data);
    context.primaryOffer = offer;
    context.primarySelectionReceived = true;
}

const ext_data_control_device_v1_listener DeviceListener = {
    .data_offer = deviceDataOffer,
    .selection = deviceSelection,
    .finished = deviceFinished,
    .primary_selection = devicePrimarySelection,
};

} // namespace

WaylandDataControlSource::WaylandDataControlSource(std::chrono::milliseconds ioTimeout)
    : m_ioTimeout(std::max(ioTimeout, std::chrono::milliseconds(1))) {}

PrimarySelectionReadResult WaylandDataControlSource::readPrimarySelection(const QStringList& mimeTypes) {
    DisplayPtr display(wl_display_connect(nullptr));
    if (!display) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not connect to the Wayland display."),
                       QStringLiteral("Wayland primary-selection retrieval requires a Wayland session."));
    }

    ReadContext context;
    context.registry = wl_display_get_registry(display.get());
    if (!context.registry) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not access the Wayland registry."));
    }

    if (wl_registry_add_listener(context.registry, &RegistryListener, &context) != 0) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not listen to the Wayland registry."));
    }

    if (wl_display_roundtrip(display.get()) < 0) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("The Wayland display connection failed."));
    }

    if (!context.manager) {
        return failure(ErrorCode::SelectionUnsupported,
                       QStringLiteral("The compositor does not support automatic selection retrieval."),
                       QStringLiteral("Wayland selected-text lookup requires ext_data_control_manager_v1."));
    }

    if (!context.seat) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("The Wayland session has no input seat."));
    }

    context.device = ext_data_control_manager_v1_get_data_device(context.manager, context.seat);
    if (!context.device) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not create a Wayland data device."));
    }

    if (ext_data_control_device_v1_add_listener(context.device, &DeviceListener, &context) != 0) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not listen to the Wayland data device."));
    }

    if (wl_display_roundtrip(display.get()) < 0) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("The Wayland display connection failed."));
    }

    if (!waitForPrimarySelection(display.get(), context, m_ioTimeout)) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("No text is currently selected."),
                       QStringLiteral("The compositor did not report a primary selection in time."));
    }

    if (!context.primaryOffer) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("No text is currently selected."));
    }

    const QStringList offeredTypes = context.offers.value(context.primaryOffer).mimeTypes;
    const std::optional<QString> chosenType = chooseMimeType(mimeTypes, offeredTypes);
    if (!chosenType) {
        return failure(ErrorCode::SelectionNotText,
                       QStringLiteral("The current selection does not contain plain text."),
                       QStringLiteral("Offered types: %1").arg(offeredTypes.join(QStringLiteral(", "))));
    }

    std::array<int, 2> pipeDescriptors = {-1, -1};
    if (::pipe(pipeDescriptors.data()) != 0) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not create a pipe for the selection."),
                       systemErrorMessage(errno));
    }

    const auto closeReadDescriptor = qScopeGuard([&pipeDescriptors] { ::close(pipeDescriptors[0]); });
    const QByteArray mimeType = chosenType->toUtf8();
    ext_data_control_offer_v1_receive(context.primaryOffer, mimeType.constData(), pipeDescriptors[1]);
    const bool flushed = flushDisplay(display.get(), m_ioTimeout);
    ::close(pipeDescriptors[1]);
    if (!flushed) {
        return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not request the selected text."),
                       systemErrorMessage(errno));
    }

    QByteArray data;
    const auto deadline = std::chrono::steady_clock::now() + m_ioTimeout;
    std::array<char, 4096> buffer{};
    while (true) {
        const auto remaining = remainingTime(deadline);
        if (remaining.count() == 0) {
            return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Timed out while reading the selection."));
        }

        pollfd descriptor{.fd = pipeDescriptors[0], .events = POLLIN, .revents = 0};
        const int result = ::poll(&descriptor, 1, pollTimeout(remaining));
        if (result < 0) {
            if (errno == EINTR) {
                continue;
            }
            return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not read the selection."),
                           systemErrorMessage(errno));
        }
        if (result == 0) {
            return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Timed out while reading the selection."));
        }

        const ssize_t count = ::read(pipeDescriptors[0], buffer.data(), buffer.size());
        if (count > 0) {
            data.append(buffer.data(), static_cast<int>(count));
            if (data.size() > MaxReceivedBytes) {
                break;
            }
        } else if (count == 0) {
            break;
        } else if (errno != EINTR) {
            return failure(ErrorCode::SelectionUnavailable, QStringLiteral("Could not read the selection."),
                           systemErrorMessage(errno));
        }
    }

    return PrimarySelectionReadResult::success(*chosenType, std::move(data));
}

QString WaylandDataControlSource::displayName() const {
    return QStringLiteral("Wayland primary selection (ext-data-control-v1)");
}

} // namespace Lexicon

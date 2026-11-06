#include "infrastructure/UnixSignalHandler.h"

#include "infrastructure/logging/Logging.h"

#include <QSocketNotifier>

#include <array>
#include <csignal>
#include <fcntl.h>
#include <sys/socket.h>
#include <unistd.h>

namespace Lexicon {

int UnixSignalHandler::s_sharedWriteFd = -1;

namespace {

void restoreSignal(int signalNumber) {
    ::signal(signalNumber, SIG_DFL);
}

} // namespace

int UnixSignalHandler::sharedWriteFd() {
    return UnixSignalHandler::s_sharedWriteFd;
}

void UnixSignalHandler::setSharedWriteFd(int fd) {
    UnixSignalHandler::s_sharedWriteFd = fd;
}

void UnixSignalHandler::clearSharedWriteFd() {
    UnixSignalHandler::s_sharedWriteFd = -1;
}

UnixSignalHandler::UnixSignalHandler(QObject* parent) : QObject(parent) {
    std::array<int, 2> fds = {-1, -1};
#ifdef SOCK_CLOEXEC
    const int socketType = SOCK_STREAM | SOCK_CLOEXEC;
#else
    const int socketType = SOCK_STREAM;
#endif
    if (::socketpair(AF_UNIX, socketType, 0, fds.data()) != 0) {
        qCWarning(lexiconApp) << "event=signal_pipe_unavailable";
        return;
    }

    m_readFd = fds[0];
    m_writeFd = fds[1];
    setSharedWriteFd(m_writeFd);

    const int flags = ::fcntl(m_readFd, F_GETFL, 0);
    if (flags >= 0) {
        const unsigned int updatedFlags = static_cast<unsigned int>(flags) | static_cast<unsigned int>(O_NONBLOCK);
        ::fcntl(m_readFd, F_SETFL, static_cast<int>(updatedFlags));
    }

    if (m_readFd >= 0) {
        m_notifier = new QSocketNotifier(m_readFd, QSocketNotifier::Read, this);
        connect(m_notifier, &QSocketNotifier::activated, this, [this] {
            std::array<char, 64> buffer{};
            while (::read(m_readFd, buffer.data(), buffer.size()) > 0) {
            }
            Q_EMIT terminationRequested();
        });
    }

    struct sigaction action = {};
    action.sa_handler = [](int signalNumber) {
        if (signalNumber == SIGINT || signalNumber == SIGTERM) {
            if (UnixSignalHandler::sharedWriteFd() >= 0) {
                const char byte = static_cast<char>(signalNumber);
                const ssize_t ignored = ::write(UnixSignalHandler::sharedWriteFd(), &byte, sizeof(byte));
                (void)ignored;
            }
        }
    };
    sigemptyset(&action.sa_mask);
    action.sa_flags = SA_RESTART;
    ::sigaction(SIGINT, &action, nullptr);
    ::sigaction(SIGTERM, &action, nullptr);
}

UnixSignalHandler::~UnixSignalHandler() {
    restoreSignal(SIGINT);
    restoreSignal(SIGTERM);

    clearSharedWriteFd();

    if (m_readFd >= 0) {
        ::close(m_readFd);
    }
    if (m_writeFd >= 0) {
        ::close(m_writeFd);
    }
    m_readFd = -1;
    m_writeFd = -1;
}

} // namespace Lexicon

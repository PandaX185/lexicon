#pragma once

#include <QObject>

class QSocketNotifier;

namespace Lexicon {

class UnixSignalHandler final : public QObject {
    Q_OBJECT

  public:
    explicit UnixSignalHandler(QObject* parent = nullptr);
    ~UnixSignalHandler() override;

    // Signal-safe accessors used by the registered POSIX signal handler.
    static int sharedWriteFd();
    static void setSharedWriteFd(int fd);
    static void clearSharedWriteFd();

  Q_SIGNALS:
    void terminationRequested();

  private:
    int m_readFd = -1;
    int m_writeFd = -1;
    QSocketNotifier* m_notifier = nullptr;

    static int s_sharedWriteFd;
};

} // namespace Lexicon

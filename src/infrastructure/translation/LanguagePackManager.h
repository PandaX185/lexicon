#pragma once

#include <QByteArray>
#include <QHash>
#include <QList>
#include <QMetaType>
#include <QObject>
#include <QString>
#include <QUrl>

class QNetworkAccessManager;
class QNetworkReply;

namespace Lexicon {

// One downloadable translation model. `url` points at the model archive and
// `sha256` is the hex digest the download is verified against before it is
// installed; a pack without a digest is never installed.
struct LanguagePackInfo {
    QString id;
    QString source;
    QString target;
    QString version;
    QString url;
    QString sha256;
    QString description;
    qint64 size = 0;

    [[nodiscard]] bool isValid() const {
        return !id.isEmpty() && !source.isEmpty() && !target.isEmpty() && !url.isEmpty() && !sha256.isEmpty();
    }
};

// Downloads language packs and the catalog that lists them. This is the only
// component allowed to touch the network, and it only ever runs because the user
// asked for a pack: nothing here is triggered on startup or during a lookup.
class LanguagePackManager : public QObject {
    Q_OBJECT

  public:
    explicit LanguagePackManager(QObject* parent = nullptr);
    ~LanguagePackManager() override;

    // The catalog is a small static JSON document, so it can be hosted anywhere.
    static QList<LanguagePackInfo> parseCatalogJson(const QByteArray& json);
    static bool verifyFileIntegrity(const QString& filePath, const QString& expectedSha256);

    void fetchCatalog(const QUrl& catalogUrl);
    void installPack(const LanguagePackInfo& pack);
    void cancelDownload(const QString& packId);

  Q_SIGNALS:
    void catalogLoaded(const QList<Lexicon::LanguagePackInfo>& packs);
    void catalogFailed(const QString& message);
    void downloadProgress(const QString& packId, qint64 received, qint64 total);
    void packReady(const QString& packId, const QString& filePath);
    void operationError(const QString& packId, const QString& message);

  private:
    QNetworkAccessManager* m_network;
    QHash<QString, QNetworkReply*> m_downloads;
};

} // namespace Lexicon

Q_DECLARE_METATYPE(Lexicon::LanguagePackInfo)

#include "infrastructure/translation/LanguagePackManager.h"

#include "infrastructure/logging/Logging.h"

#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QRegularExpression>
#include <QStandardPaths>

#include <utility>

namespace Lexicon {
namespace {

constexpr int CatalogVersion = 1;

QNetworkRequest catalogRequest(const QUrl& url) {
    QNetworkRequest request(url);
    // A redirect is expected when the catalog is served by a release host.
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::NoLessSafeRedirectPolicy);
    request.setHeader(QNetworkRequest::UserAgentHeader, QStringLiteral("Lexicon"));
    return request;
}

QString downloadPathFor(const LanguagePackInfo& pack) {
    const QString directory =
        QStandardPaths::writableLocation(QStandardPaths::TempLocation) + QStringLiteral("/lexicon-language-packs");
    QDir().mkpath(directory);
    QString name = pack.id;
    name.replace(QRegularExpression(QStringLiteral("[^A-Za-z0-9._-]")), QStringLiteral("_"));
    return directory + QLatin1Char('/') + name + QStringLiteral(".argosmodel");
}

} // namespace

LanguagePackManager::LanguagePackManager(QObject* parent)
    : QObject(parent), m_network(new QNetworkAccessManager(this)) {
    qRegisterMetaType<Lexicon::LanguagePackInfo>();
}

LanguagePackManager::~LanguagePackManager() {
    for (QNetworkReply* reply : std::as_const(m_downloads)) {
        reply->abort();
    }
    m_downloads.clear();
}

QList<LanguagePackInfo> LanguagePackManager::parseCatalogJson(const QByteArray& json) {
    QList<LanguagePackInfo> packs;

    QJsonParseError error;
    const QJsonDocument document = QJsonDocument::fromJson(json, &error);
    if (document.isNull() || !document.isObject()) {
        return packs;
    }

    const QJsonObject root = document.object();
    if (root.value(QStringLiteral("version")).toInt(CatalogVersion) > CatalogVersion) {
        return packs;
    }

    const QJsonArray array = root.value(QStringLiteral("packs")).toArray();
    packs.reserve(array.size());
    for (const auto& value : array) {
        const QJsonObject item = value.toObject();
        LanguagePackInfo pack{
            .id = item.value(QStringLiteral("id")).toString(),
            .source = item.value(QStringLiteral("source")).toString().toLower(),
            .target = item.value(QStringLiteral("target")).toString().toLower(),
            .version = item.value(QStringLiteral("version")).toString(),
            .url = item.value(QStringLiteral("url")).toString(),
            .sha256 = item.value(QStringLiteral("sha256")).toString().toLower(),
            .description = item.value(QStringLiteral("description")).toString(),
            .size = static_cast<qint64>(item.value(QStringLiteral("size")).toDouble()),
        };
        if (pack.isValid() && LanguagePackManager::isSecureUrl(QUrl(pack.url))) {
            packs.append(std::move(pack));
        }
    }
    return packs;
}

bool LanguagePackManager::isSecureUrl(const QUrl& url) {
    return url.isValid() && url.scheme().compare(QStringLiteral("https"), Qt::CaseInsensitive) == 0;
}

bool LanguagePackManager::verifyFileIntegrity(const QString& filePath, const QString& expectedSha256) {
    if (expectedSha256.isEmpty()) {
        return false;
    }
    QFile file(filePath);
    if (!file.open(QIODevice::ReadOnly)) {
        return false;
    }

    QCryptographicHash hash(QCryptographicHash::Sha256);
    if (!hash.addData(&file)) {
        return false;
    }
    return QString::fromLatin1(hash.result().toHex()).compare(expectedSha256, Qt::CaseInsensitive) == 0;
}

void LanguagePackManager::fetchCatalog(const QUrl& catalogUrl) {
    if (!catalogUrl.isValid()) {
        Q_EMIT catalogFailed(QStringLiteral("The language pack catalog location is not configured."));
        return;
    }
    if (!isSecureUrl(catalogUrl)) {
        Q_EMIT catalogFailed(QStringLiteral("The language pack catalog must use HTTPS."));
        return;
    }

    QNetworkReply* reply = m_network->get(catalogRequest(catalogUrl));
    QObject::connect(reply, &QNetworkReply::finished, this, [this, reply] {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            qCInfo(lexiconApp) << "event=language_pack_catalog_failed error=" << reply->errorString();
            Q_EMIT catalogFailed(reply->errorString());
            return;
        }
        const QList<LanguagePackInfo> packs = parseCatalogJson(reply->readAll());
        qCInfo(lexiconApp) << "event=language_pack_catalog_loaded packs=" << packs.size();
        Q_EMIT catalogLoaded(packs);
    });
}

void LanguagePackManager::installPack(const LanguagePackInfo& pack) {
    if (!pack.isValid()) {
        Q_EMIT operationError(pack.id, QStringLiteral("This language pack entry is incomplete."));
        return;
    }
    if (!isSecureUrl(QUrl(pack.url))) {
        Q_EMIT operationError(pack.id, QStringLiteral("Language packs must be downloaded over HTTPS."));
        return;
    }
    if (m_downloads.contains(pack.id)) {
        Q_EMIT operationError(pack.id, QStringLiteral("This language pack is already downloading."));
        return;
    }

    const QString path = downloadPathFor(pack);
    auto* file = new QFile(path, this);
    if (!file->open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        const QString error = file->errorString();
        delete file;
        Q_EMIT operationError(pack.id, error);
        return;
    }

    QNetworkReply* reply = m_network->get(catalogRequest(QUrl(pack.url)));
    m_downloads.insert(pack.id, reply);

    QObject::connect(reply, &QNetworkReply::readyRead, this, [reply, file] { file->write(reply->readAll()); });
    QObject::connect(
        reply, &QNetworkReply::downloadProgress, this,
        [this, id = pack.id](qint64 received, qint64 total) { Q_EMIT downloadProgress(id, received, total); });
    QObject::connect(reply, &QNetworkReply::finished, this, [this, reply, file, pack, path] {
        m_downloads.remove(pack.id);
        file->write(reply->readAll());
        file->close();
        file->deleteLater();
        reply->deleteLater();

        if (reply->error() != QNetworkReply::NoError) {
            QFile::remove(path);
            if (reply->error() == QNetworkReply::OperationCanceledError) {
                qCInfo(lexiconApp) << "event=language_pack_download_cancelled pack=" << pack.id;
                return;
            }
            qCWarning(lexiconApp) << "event=language_pack_download_failed pack=" << pack.id
                                  << "error=" << reply->errorString();
            Q_EMIT operationError(pack.id, reply->errorString());
            return;
        }

        if (!verifyFileIntegrity(path, pack.sha256)) {
            QFile::remove(path);
            qCWarning(lexiconApp) << "event=language_pack_checksum_mismatch pack=" << pack.id;
            Q_EMIT operationError(pack.id, QStringLiteral("The downloaded language pack failed verification."));
            return;
        }

        qCInfo(lexiconApp) << "event=language_pack_downloaded pack=" << pack.id << "bytes=" << QFileInfo(path).size();
        Q_EMIT packReady(pack.id, path);
    });
}

void LanguagePackManager::cancelDownload(const QString& packId) {
    if (QNetworkReply* reply = m_downloads.value(packId, nullptr)) {
        reply->abort();
    }
}

} // namespace Lexicon

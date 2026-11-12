#include "app/LanguagePackController.h"

#include "core/models/LanguagePair.h"
#include "infrastructure/logging/Logging.h"
#include "infrastructure/translation/ArgosTranslationService.h"
#include "infrastructure/translation/LanguagePackManager.h"

#include <QFile>
#include <QFutureWatcher>
#include <QPointer>
#include <QtConcurrent/QtConcurrentRun>

#include <utility>

namespace Lexicon {
namespace {

struct InstalledPairsOutcome {
    QList<LanguagePair> pairs;
    QString error;
};

[[nodiscard]] QVariantMap pairToVariant(const LanguagePair& pair) {
    QVariantMap map;
    const QString source = pair.sourceName.isEmpty() ? pair.source : pair.sourceName;
    const QString target = pair.targetName.isEmpty() ? pair.target : pair.targetName;
    map.insert(QStringLiteral("source"), pair.source);
    map.insert(QStringLiteral("target"), pair.target);
    map.insert(QStringLiteral("label"), QStringLiteral("%1 → %2").arg(source, target));
    return map;
}

[[nodiscard]] QVariantMap packToVariant(const LanguagePackInfo& pack) {
    QVariantMap map;
    map.insert(QStringLiteral("id"), pack.id);
    map.insert(QStringLiteral("source"), pack.source);
    map.insert(QStringLiteral("target"), pack.target);
    map.insert(QStringLiteral("label"), QStringLiteral("%1 → %2").arg(pack.source.toUpper(), pack.target));
    map.insert(QStringLiteral("version"), pack.version);
    map.insert(QStringLiteral("description"), pack.description);
    map.insert(QStringLiteral("size"), pack.size);
    return map;
}

} // namespace

LanguagePackController::LanguagePackController(Config config, QObject* parent)
    : QObject(parent), m_config(std::move(config)), m_translation(std::make_unique<ArgosTranslationService>()),
      m_packs(std::make_unique<LanguagePackManager>()) {
    QObject::connect(m_packs.get(), &LanguagePackManager::catalogLoaded, this,
                     [this](const QList<LanguagePackInfo>& packs) {
                         m_catalog.clear();
                         m_availablePacks.clear();
                         for (const LanguagePackInfo& pack : packs) {
                             m_catalog.insert(pack.id, pack);
                             m_availablePacks.append(packToVariant(pack));
                         }
                         Q_EMIT availablePacksChanged();
                         setStatus(packs.isEmpty() ? QStringLiteral("The catalog lists no language packs.")
                                                   : QStringLiteral("%1 language packs available.").arg(packs.size()));
                     });

    QObject::connect(m_packs.get(), &LanguagePackManager::catalogFailed, this, [this](const QString& message) {
        setStatus(QStringLiteral("Could not load the language pack catalog: %1").arg(message));
    });

    QObject::connect(m_packs.get(), &LanguagePackManager::downloadProgress, this,
                     [this](const QString& packId, qint64 received, qint64 total) {
                         m_activePack = packId;
                         m_progress = total > 0 ? static_cast<qreal>(received) / static_cast<qreal>(total) : 0.0;
                         Q_EMIT activePackChanged();
                         Q_EMIT progressChanged();
                     });

    QObject::connect(m_packs.get(), &LanguagePackManager::operationError, this,
                     [this](const QString& packId, const QString& message) {
                         setBusy(false);
                         m_activePack = packId;
                         m_progress = 0.0;
                         Q_EMIT activePackChanged();
                         Q_EMIT progressChanged();
                         setStatus(message);
                     });

    QObject::connect(m_packs.get(), &LanguagePackManager::packReady, this,
                     [this](const QString& packId, const QString& filePath) {
                         setStatus(QStringLiteral("Installing %1…").arg(packId));
                         ArgosTranslationService* translation = m_translation.get();
                         const QPointer<LanguagePackController> self(this);

                         auto* watcher = new QFutureWatcher<QString>(this);
                         QObject::connect(watcher, &QFutureWatcher<QString>::finished, this, [self, watcher, filePath] {
                             const QString error = watcher->result();
                             watcher->deleteLater();
                             QFile::remove(filePath);
                             if (!self) {
                                 return;
                             }
                             self->setBusy(false);
                             self->setStatus(error.isEmpty() ? QStringLiteral("Language pack installed.") : error);
                             self->refreshInstalled();
                         });

                         watcher->setFuture(QtConcurrent::run([translation, filePath]() -> QString {
                             // The translation runtime installs from the verified
                             // local archive; nothing is fetched here.
                             const Result<bool> response = translation->installFromArchive(filePath);
                             return response.hasValue() ? QString() : response.error().message;
                         }));
                     });

    setStatus(catalogConfigured() ? QStringLiteral("Loading the language pack catalog…")
                                  : QStringLiteral("No language pack catalog is configured."));
    QMetaObject::invokeMethod(this, &LanguagePackController::refreshInstalled, Qt::QueuedConnection);
}

LanguagePackController::~LanguagePackController() = default;

QVariantList LanguagePackController::installedPairs() const {
    return m_installedPairs;
}

QVariantList LanguagePackController::availablePacks() const {
    return m_availablePacks;
}

QString LanguagePackController::status() const {
    return m_status;
}

bool LanguagePackController::busy() const {
    return m_busy;
}

QString LanguagePackController::activePack() const {
    return m_activePack;
}

qreal LanguagePackController::progress() const {
    return m_progress;
}

QString LanguagePackController::catalogUrl() const {
    return m_config.translation.catalogUrl;
}

bool LanguagePackController::catalogConfigured() const {
    return !m_config.translation.catalogUrl.isEmpty();
}

void LanguagePackController::setStatus(const QString& status) {
    if (m_status == status) {
        return;
    }
    m_status = status;
    Q_EMIT statusChanged();
}

void LanguagePackController::setBusy(bool busy) {
    if (m_busy == busy) {
        return;
    }
    m_busy = busy;
    Q_EMIT busyChanged();
}

const LanguagePackInfo* LanguagePackController::findPack(const QString& packId) const {
    // The QML list keeps only display data, so the catalog is consulted again
    // for the URL and checksum when the user installs a pack.
    const auto it = m_catalog.constFind(packId);
    return it == m_catalog.cend() ? nullptr : &it.value();
}

void LanguagePackController::refresh() {
    if (catalogConfigured()) {
        setStatus(QStringLiteral("Loading the language pack catalog…"));
        m_packs->fetchCatalog(QUrl(m_config.translation.catalogUrl));
    }
    refreshInstalled();
}

void LanguagePackController::refreshInstalled() {
    ArgosTranslationService* translation = m_translation.get();
    auto* watcher = new QFutureWatcher<InstalledPairsOutcome>(this);
    const QPointer<LanguagePackController> self(this);

    QObject::connect(watcher, &QFutureWatcher<InstalledPairsOutcome>::finished, this, [self, watcher] {
        const InstalledPairsOutcome outcome = watcher->result();
        watcher->deleteLater();
        if (!self) {
            return;
        }
        self->m_installedPairs.clear();
        for (const LanguagePair& pair : outcome.pairs) {
            self->m_installedPairs.append(pairToVariant(pair));
        }
        Q_EMIT self->installedPairsChanged();
        if (!outcome.error.isEmpty()) {
            self->setStatus(outcome.error);
        }
    });

    watcher->setFuture(QtConcurrent::run([translation]() -> InstalledPairsOutcome {
        const Result<QList<LanguagePair>> pairs = translation->installedPairs();
        if (!pairs.hasValue()) {
            return InstalledPairsOutcome{.pairs = {}, .error = pairs.error().message};
        }
        return InstalledPairsOutcome{.pairs = pairs.value(), .error = {}};
    }));
}

void LanguagePackController::install(const QString& packId) {
    if (m_busy) {
        return;
    }
    const LanguagePackInfo* pack = findPack(packId);
    if (pack == nullptr) {
        setStatus(QStringLiteral("That language pack is not in the catalog."));
        return;
    }

    setBusy(true);
    m_activePack = packId;
    m_progress = 0.0;
    Q_EMIT activePackChanged();
    Q_EMIT progressChanged();
    setStatus(QStringLiteral("Downloading %1…").arg(packId));
    m_packs->installPack(*pack);
}

void LanguagePackController::remove(const QString& source, const QString& target) {
    if (m_busy) {
        return;
    }

    setBusy(true);
    setStatus(QStringLiteral("Removing %1 → %2…").arg(source, target));

    ArgosTranslationService* translation = m_translation.get();
    const QPointer<LanguagePackController> self(this);
    auto* watcher = new QFutureWatcher<QString>(this);
    QObject::connect(watcher, &QFutureWatcher<QString>::finished, this, [self, watcher] {
        const QString error = watcher->result();
        watcher->deleteLater();
        if (!self) {
            return;
        }
        self->setBusy(false);
        self->setStatus(error.isEmpty() ? QStringLiteral("Language pack removed.") : error);
        self->refreshInstalled();
    });

    watcher->setFuture(QtConcurrent::run([translation, source, target]() -> QString {
        const Result<bool> response = translation->uninstallPair(source, target);
        return response.hasValue() ? QString() : response.error().message;
    }));
}

} // namespace Lexicon

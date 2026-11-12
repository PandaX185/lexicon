#pragma once

#include "config/Config.h"
#include "infrastructure/translation/LanguagePackManager.h"

#include <QHash>
#include <QObject>
#include <QString>
#include <QVariantList>

#include <memory>

namespace Lexicon {

class ArgosTranslationService;

// Exposes the optional translation packs to QML. Installing a pack is the only
// path that downloads anything, it always starts from an explicit user action,
// and the download is verified before the translation runtime installs it.
class LanguagePackController final : public QObject {
    Q_OBJECT

    Q_PROPERTY(QVariantList installedPairs READ installedPairs NOTIFY installedPairsChanged)
    Q_PROPERTY(QVariantList availablePacks READ availablePacks NOTIFY availablePacksChanged)
    Q_PROPERTY(QString status READ status NOTIFY statusChanged)
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(QString activePack READ activePack NOTIFY activePackChanged)
    Q_PROPERTY(qreal progress READ progress NOTIFY progressChanged)
    Q_PROPERTY(QString catalogUrl READ catalogUrl CONSTANT)
    Q_PROPERTY(bool catalogConfigured READ catalogConfigured CONSTANT)

  public:
    explicit LanguagePackController(Config config, QObject* parent = nullptr);
    ~LanguagePackController() override;

    [[nodiscard]] QVariantList installedPairs() const;
    [[nodiscard]] QVariantList availablePacks() const;
    [[nodiscard]] QString status() const;
    [[nodiscard]] bool busy() const;
    [[nodiscard]] QString activePack() const;
    [[nodiscard]] qreal progress() const;
    [[nodiscard]] QString catalogUrl() const;
    [[nodiscard]] bool catalogConfigured() const;

    // Reloads the catalog and the installed pairs. `refreshInstalled()` alone is
    // used after an install or removal, when the catalog has not changed.
    Q_INVOKABLE void refresh();
    Q_INVOKABLE void install(const QString& packId);
    Q_INVOKABLE void remove(const QString& source, const QString& target);

  Q_SIGNALS:
    void installedPairsChanged();
    void availablePacksChanged();
    void statusChanged();
    void busyChanged();
    void activePackChanged();
    void progressChanged();

  private:
    void refreshInstalled();
    void setStatus(const QString& status);
    void setBusy(bool busy);
    const LanguagePackInfo* findPack(const QString& packId) const;

    Config m_config;
    std::unique_ptr<ArgosTranslationService> m_translation;
    std::unique_ptr<LanguagePackManager> m_packs;
    QHash<QString, LanguagePackInfo> m_catalog;
    QVariantList m_installedPairs;
    QVariantList m_availablePacks;
    QString m_status;
    QString m_activePack;
    qreal m_progress = 0.0;
    bool m_busy = false;

    Q_DISABLE_COPY_MOVE(LanguagePackController)
};

} // namespace Lexicon

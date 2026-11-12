#pragma once

#include "core/interfaces/TranslationService.h"
#include "infrastructure/translation/ArgosProcessWorker.h"

#include <memory>

namespace Lexicon {

// Translation through the local Argos Translate worker. The worker is started on
// the first request and reused afterwards; calls block, so they belong on a
// worker thread. Nothing here touches the network.
class ArgosTranslationService final : public TranslationService {
  public:
    explicit ArgosTranslationService(std::unique_ptr<ArgosProcessWorker> worker = {});
    ~ArgosTranslationService() override;

    Result<QString> translate(const QString& text, const QString& from, const QString& to) override;
    Result<QList<LanguagePair>> installedPairs() override;
    bool isPairInstalled(const QString& from, const QString& to) override;

    // Pack management is backend-specific, so it is not part of the interface:
    // callers that reach for it already know which runtime they installed.
    Result<bool> installFromArchive(const QString& archivePath);
    Result<bool> uninstallPair(const QString& from, const QString& to);

  private:
    std::unique_ptr<ArgosProcessWorker> m_worker;
};

} // namespace Lexicon

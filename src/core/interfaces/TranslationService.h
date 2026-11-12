#pragma once

#include "core/models/LanguagePair.h"
#include "core/types/Result.h"

#include <QList>
#include <QString>

namespace Lexicon {

// Translation is a separate, optional capability: the dictionary works with no
// translation backend at all, and a backend can be swapped without touching the
// dictionary, the UI, or the controller.
class TranslationService {
  public:
    virtual ~TranslationService() = default;

    // Both calls block on the backend, so they are used from worker threads.
    virtual Result<QString> translate(const QString& text, const QString& from, const QString& to) = 0;
    virtual Result<QList<LanguagePair>> installedPairs() = 0;

    virtual bool isPairInstalled(const QString& from, const QString& to) = 0;
};

} // namespace Lexicon

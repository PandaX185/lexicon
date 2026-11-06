#pragma once

#include <QLoggingCategory>

Q_DECLARE_LOGGING_CATEGORY(lexiconApp)
Q_DECLARE_LOGGING_CATEGORY(lexiconConfig)
Q_DECLARE_LOGGING_CATEGORY(lexiconDictionary)
Q_DECLARE_LOGGING_CATEGORY(lexiconDatabase)
Q_DECLARE_LOGGING_CATEGORY(lexiconKde)
Q_DECLARE_LOGGING_CATEGORY(lexiconUi)

namespace Lexicon::Logging {

void initialize(bool debugEnabled);

} // namespace Lexicon::Logging

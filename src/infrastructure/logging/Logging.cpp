#include "infrastructure/logging/Logging.h"

Q_LOGGING_CATEGORY(lexiconApp, "lexicon.app")
Q_LOGGING_CATEGORY(lexiconConfig, "lexicon.config")
Q_LOGGING_CATEGORY(lexiconDictionary, "lexicon.dictionary")
Q_LOGGING_CATEGORY(lexiconDatabase, "lexicon.database")
Q_LOGGING_CATEGORY(lexiconKde, "lexicon.kde")
Q_LOGGING_CATEGORY(lexiconUi, "lexicon.ui")

namespace Lexicon::Logging {

void initialize(bool debugEnabled) {
    qSetMessagePattern(QStringLiteral("%{time yyyy-MM-ddTHH:mm:ss.zzz} %{category} %{type} %{message}"));
    if (debugEnabled) {
        QLoggingCategory::setFilterRules(QStringLiteral("lexicon.*.debug=true\nlexicon.*.info=true"));
    } else {
        QLoggingCategory::setFilterRules(QStringLiteral("lexicon.*.debug=false"));
    }
}

} // namespace Lexicon::Logging

#pragma once

#include "dictionary/DictionaryRepository.h"

#include <QString>

namespace Lexicon {

class SqliteDictionaryRepository final : public DictionaryRepository {
  public:
    explicit SqliteDictionaryRepository(QString databasePath);

    [[nodiscard]] DictionaryLookupResult findByWord(const QString& word) const override;
    [[nodiscard]] QString location() const override;

  private:
    QString m_databasePath;
};

} // namespace Lexicon

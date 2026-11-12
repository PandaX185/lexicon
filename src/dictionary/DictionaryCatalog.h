#pragma once

#include "dictionary/DictionaryRepository.h"
#include "dictionary/models/DictionaryInfo.h"

#include <QList>
#include <QString>

#include <functional>
#include <memory>
#include <optional>

namespace Lexicon {

struct DictionaryCatalogEntry {
    DictionaryInfo info;
    QString path;
    qint64 entryCount = 0;
};

// The dictionaries installed in one directory. Building a catalog scans the
// directory once and asks every database to describe itself; files that are not
// Lexicon dictionaries are skipped. A catalog is a snapshot, so it can be read
// from worker threads after construction, and `refresh()` picks up content that
// was installed later.
class DictionaryCatalog {
  public:
    using RepositoryFactory = std::function<std::unique_ptr<DictionaryRepository>(const QString& path)>;

    explicit DictionaryCatalog(QString directory, RepositoryFactory factory = {});
    explicit DictionaryCatalog(QStringList directories, RepositoryFactory factory = {});

    void refresh();

    [[nodiscard]] const QList<DictionaryCatalogEntry>& entries() const noexcept;
    [[nodiscard]] QString directory() const;
    [[nodiscard]] QStringList directories() const;

    // Resolution order: the exact language pair, then a monolingual dictionary
    // for the source language, then one for the target language. A pair that is
    // not installed yields an empty result rather than a wrong-language answer.
    [[nodiscard]] std::optional<DictionaryCatalogEntry> find(const QString& sourceLanguage,
                                                             const QString& targetLanguage) const;

  private:
    QStringList m_directories;
    RepositoryFactory m_factory;
    QList<DictionaryCatalogEntry> m_entries;
};

} // namespace Lexicon

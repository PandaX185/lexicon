#include "dictionary/DictionaryCatalog.h"

#include <QDir>
#include <QFileInfo>
#include <QSet>

#include <algorithm>
#include <utility>

namespace Lexicon {

DictionaryCatalog::DictionaryCatalog(QString directory, RepositoryFactory factory)
    : DictionaryCatalog(QStringList{std::move(directory)}, std::move(factory)) {}

DictionaryCatalog::DictionaryCatalog(QStringList directories, RepositoryFactory factory)
    : m_directories(std::move(directories)), m_factory(std::move(factory)) {
    refresh();
}

void DictionaryCatalog::refresh() {
    QList<DictionaryCatalogEntry> entries;
    QSet<QString> seenFileNames;
    if (m_factory) {
        for (const QString& dirPath : m_directories) {
            const QDir directory(dirPath);
            const QFileInfoList files =
                directory.entryInfoList({QStringLiteral("*.sqlite")}, QDir::Files | QDir::Readable, QDir::Name);
            for (const QFileInfo& file : files) {
                const QString fileName = file.fileName();
                if (seenFileNames.contains(fileName)) {
                    continue;
                }
                const QString path = file.absoluteFilePath();
                const std::unique_ptr<DictionaryRepository> repository = m_factory(path);
                if (!repository) {
                    continue;
                }
                const DictionaryDescription description = repository->describe();
                if (!description.ok() || !description.info.isValid()) {
                    continue;
                }
                seenFileNames.insert(fileName);
                entries.append(DictionaryCatalogEntry{
                    .info = description.info,
                    .path = path,
                    .entryCount = description.entryCount,
                });
            }
        }
    }
    m_entries = std::move(entries);
}

const QList<DictionaryCatalogEntry>& DictionaryCatalog::entries() const noexcept {
    return m_entries;
}

QStringList DictionaryCatalog::directories() const {
    return m_directories;
}

QString DictionaryCatalog::directory() const {
    return m_directories.isEmpty() ? QString() : m_directories.constFirst();
}

std::optional<DictionaryCatalogEntry> DictionaryCatalog::find(const QString& sourceLanguage,
                                                              const QString& targetLanguage) const {
    const QString source = normalizeLanguageTag(sourceLanguage);
    const QString target = normalizeLanguageTag(targetLanguage);

    const auto exact = std::ranges::find_if(m_entries, [&source, &target](const DictionaryCatalogEntry& entry) {
        return entry.info.matches(source, target);
    });
    if (exact != m_entries.cend()) {
        return *exact;
    }

    // A monolingual dictionary still answers a translation request with
    // definitions in its own language, which is more useful than an empty
    // popup, as long as the popup names the dictionary it used.
    const auto monolingual = [this](const QString& language) -> std::optional<DictionaryCatalogEntry> {
        if (language.isEmpty()) {
            return std::nullopt;
        }
        const auto match = std::ranges::find_if(m_entries, [&language](const DictionaryCatalogEntry& entry) {
            return entry.info.sourceLanguage == language && entry.info.targetLanguage == language;
        });
        if (match == m_entries.cend()) {
            return std::nullopt;
        }
        return *match;
    };

    if (const auto fallback = monolingual(source)) {
        return fallback;
    }
    return monolingual(target);
}

} // namespace Lexicon

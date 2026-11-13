#include "app/DictionarySeeder.h"

#include "infrastructure/logging/Logging.h"

#include <QDir>
#include <QFile>
#include <QStandardPaths>

namespace Lexicon {
namespace {

bool directoryContainsDatabase(const QString& directory) {
    return !QDir(directory).entryList({QStringLiteral("*.sqlite")}, QDir::Files).isEmpty();
}

} // namespace

bool seedUserDictionary(const DictionarySeedPaths& paths) {
    for (const QString& directory : paths.populatedDirectories) {
        if (directoryContainsDatabase(directory)) {
            return false;
        }
    }
    if (directoryContainsDatabase(paths.destinationDirectory)) {
        return false;
    }

    QString source;
    for (const QString& directory : paths.sourceDirectories) {
        const QString candidate = directory + QStringLiteral("/en.sqlite");
        if (QFile::exists(candidate)) {
            source = candidate;
            break;
        }
    }
    if (source.isEmpty()) {
        return false;
    }

    if (!QDir().mkpath(paths.destinationDirectory)) {
        qCWarning(lexiconApp) << "event=dictionary_seed_failed reason=mkpath_failed path="
                              << paths.destinationDirectory;
        return false;
    }

    const QString destination = paths.destinationDirectory + QStringLiteral("/en.sqlite");
    if (!QFile::copy(source, destination)) {
        qCWarning(lexiconApp) << "event=dictionary_seed_failed source=" << source << "destination=" << destination;
        return false;
    }
    qCInfo(lexiconApp) << "event=dictionary_seeded source=" << source << "destination=" << destination;
    return true;
}

bool seedUserDictionary(const Config& config) {
    if (!config.usesDictionaryDirectory()) {
        return false;
    }

    const QString writableRoot = QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation);
    const QString subPath = QStringLiteral("/lexicon/dictionaries");

    QStringList sources;
    for (const QString& base : QStandardPaths::standardLocations(QStandardPaths::GenericDataLocation)) {
        sources << base + subPath;
    }
    sources << QStringLiteral("/usr/share/lexicon/dictionaries")
            << QStringLiteral("/usr/local/share/lexicon/dictionaries");

    return seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = config.resolvedDictionaryDirectories(),
        .sourceDirectories = sources,
        .destinationDirectory = writableRoot + subPath,
    });
}

} // namespace Lexicon

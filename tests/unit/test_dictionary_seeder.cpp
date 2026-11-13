#include "app/DictionarySeeder.h"
#include "support/ScopedEnvironment.h"

#include <QDir>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>

using namespace Lexicon;
using Lexicon::Test::ScopedEnvironment;

namespace {

QString writeFile(const QString& path, const QString& contents) {
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        return {};
    }
    file.write(contents.toUtf8());
    file.close();
    return path;
}

QString readFile(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly)) {
        return {};
    }
    return QString::fromUtf8(file.readAll());
}

} // namespace

class DictionarySeederTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void seedsMissingDictionary();
    void createsDestinationDirectory();
    void skipsWhenDestinationHasDatabase();
    void skipsWhenConfiguredDirectoryHasDatabase();
    void skipsWhenSourceIsMissing();
    void doesNotOverwriteExistingDestination();
    void skipsWhenExplicitDatabasePathIsConfigured();
    void seedsExplicitConfiguredDirectory();
};

void DictionarySeederTest::seedsMissingDictionary() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString sourceDir = directory.filePath(QStringLiteral("source"));
    const QString destinationDir = directory.filePath(QStringLiteral("user/dictionaries"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("dictionary")).isEmpty());

    const bool seeded = seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {},
        .sourceDirectories = {sourceDir},
        .destinationDirectory = destinationDir,
    });

    QVERIFY(seeded);
    QCOMPARE(readFile(destinationDir + QStringLiteral("/en.sqlite")), QStringLiteral("dictionary"));
}

void DictionarySeederTest::createsDestinationDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString sourceDir = directory.filePath(QStringLiteral("source"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("dictionary")).isEmpty());
    const QString destinationDir = directory.filePath(QStringLiteral("missing/nested/dir"));

    QVERIFY(seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {},
        .sourceDirectories = {sourceDir},
        .destinationDirectory = destinationDir,
    }));
    QVERIFY(QFile::exists(destinationDir + QStringLiteral("/en.sqlite")));
}

void DictionarySeederTest::skipsWhenDestinationHasDatabase() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString sourceDir = directory.filePath(QStringLiteral("source"));
    const QString destinationDir = directory.filePath(QStringLiteral("user"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(QDir().mkpath(destinationDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("new")).isEmpty());
    QVERIFY(!writeFile(destinationDir + QStringLiteral("/custom.sqlite"), QStringLiteral("installed")).isEmpty());

    QVERIFY(!seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {},
        .sourceDirectories = {sourceDir},
        .destinationDirectory = destinationDir,
    }));
    QVERIFY(!QFile::exists(destinationDir + QStringLiteral("/en.sqlite")));
}

void DictionarySeederTest::skipsWhenConfiguredDirectoryHasDatabase() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString sourceDir = directory.filePath(QStringLiteral("source"));
    const QString populatedDir = directory.filePath(QStringLiteral("populated"));
    const QString destinationDir = directory.filePath(QStringLiteral("user"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(QDir().mkpath(populatedDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("new")).isEmpty());
    QVERIFY(!writeFile(populatedDir + QStringLiteral("/dictionary.sqlite"), QStringLiteral("installed")).isEmpty());

    QVERIFY(!seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {populatedDir},
        .sourceDirectories = {sourceDir},
        .destinationDirectory = destinationDir,
    }));
    QVERIFY(!QFile::exists(destinationDir + QStringLiteral("/en.sqlite")));
}

void DictionarySeederTest::skipsWhenSourceIsMissing() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString destinationDir = directory.filePath(QStringLiteral("user"));

    QVERIFY(!seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {},
        .sourceDirectories = {directory.filePath(QStringLiteral("absent"))},
        .destinationDirectory = destinationDir,
    }));
    QVERIFY(!QFile::exists(destinationDir + QStringLiteral("/en.sqlite")));
}

void DictionarySeederTest::doesNotOverwriteExistingDestination() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString sourceDir = directory.filePath(QStringLiteral("source"));
    const QString destinationDir = directory.filePath(QStringLiteral("user"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(QDir().mkpath(destinationDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("new")).isEmpty());
    QVERIFY(!writeFile(destinationDir + QStringLiteral("/en.sqlite"), QStringLiteral("existing")).isEmpty());

    QVERIFY(!seedUserDictionary(DictionarySeedPaths{
        .populatedDirectories = {},
        .sourceDirectories = {sourceDir},
        .destinationDirectory = destinationDir,
    }));
    QCOMPARE(readFile(destinationDir + QStringLiteral("/en.sqlite")), QStringLiteral("existing"));
}

void DictionarySeederTest::skipsWhenExplicitDatabasePathIsConfigured() {
    Config config;
    config.dictionary.databasePath = QStringLiteral("/tmp/explicit.sqlite");

    QVERIFY(!seedUserDictionary(config));
}

void DictionarySeederTest::seedsExplicitConfiguredDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString dataHome = directory.filePath(QStringLiteral("data"));
    const QString sourceDir = dataHome + QStringLiteral("/lexicon/dictionaries");
    const QString destinationDir = directory.filePath(QStringLiteral("custom/dictionaries"));
    QVERIFY(QDir().mkpath(sourceDir));
    QVERIFY(!writeFile(sourceDir + QStringLiteral("/en.sqlite"), QStringLiteral("dictionary")).isEmpty());
    const ScopedEnvironment xdgDataHome("XDG_DATA_HOME", dataHome.toUtf8());

    Config config;
    config.dictionary.directory = destinationDir;
    QVERIFY(seedUserDictionary(config));
    QCOMPARE(readFile(destinationDir + QStringLiteral("/en.sqlite")), QStringLiteral("dictionary"));
}

QTEST_GUILESS_MAIN(DictionarySeederTest)

#include "test_dictionary_seeder.moc"

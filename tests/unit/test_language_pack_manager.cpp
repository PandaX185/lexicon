#include "infrastructure/translation/LanguagePackManager.h"

#include <QCryptographicHash>
#include <QTemporaryDir>
#include <QTemporaryFile>
#include <QTest>

using namespace Lexicon;

namespace {

QByteArray sha256Of(const QByteArray& data) {
    return QCryptographicHash::hash(data, QCryptographicHash::Sha256).toHex();
}

} // namespace

class LanguagePackManagerTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void parsesCatalogJson();
    void skipsIncompleteCatalogEntries();
    void rejectsUnsupportedCatalogVersion();
    void verifiesMatchingChecksum();
    void rejectsMismatchedChecksum();
    void rejectsMissingFile();
};

void LanguagePackManagerTest::parsesCatalogJson() {
    const QByteArray catalog = R"({
        "version": 1,
        "packs": [
            {
                "id": "translate-en_es",
                "source": "EN",
                "target": "es",
                "version": "1.9.0",
                "url": "https://example.com/translate-en_es.argosmodel",
                "size": 85000000,
                "sha256": "4A5B6C",
                "description": "English to Spanish"
            }
        ]
    })";

    const QList<LanguagePackInfo> packs = LanguagePackManager::parseCatalogJson(catalog);
    QCOMPARE(packs.size(), 1);
    const LanguagePackInfo& pack = packs.constFirst();
    QCOMPARE(pack.id, QStringLiteral("translate-en_es"));
    QCOMPARE(pack.source, QStringLiteral("en"));
    QCOMPARE(pack.target, QStringLiteral("es"));
    QCOMPARE(pack.sha256, QStringLiteral("4a5b6c"));
    QCOMPARE(pack.size, 85000000);
    QVERIFY(pack.isValid());
}

void LanguagePackManagerTest::skipsIncompleteCatalogEntries() {
    const QByteArray catalog = R"({
        "version": 1,
        "packs": [
            {"id": "missing-hash", "source": "en", "target": "es", "url": "https://example.com/a"},
            {"id": "missing-url", "source": "en", "target": "de", "sha256": "abcd"},
            {"id": "complete", "source": "en", "target": "fr", "url": "https://example.com/c", "sha256": "1234"}
        ]
    })";

    const QList<LanguagePackInfo> packs = LanguagePackManager::parseCatalogJson(catalog);
    QCOMPARE(packs.size(), 1);
    QCOMPARE(packs.constFirst().id, QStringLiteral("complete"));
}

void LanguagePackManagerTest::rejectsUnsupportedCatalogVersion() {
    const QByteArray catalog = R"({
        "version": 99,
        "packs": [{"id": "a", "source": "en", "target": "es", "url": "https://e/x", "sha256": "ab"}]
    })";

    QVERIFY(LanguagePackManager::parseCatalogJson(catalog).isEmpty());
    QVERIFY(LanguagePackManager::parseCatalogJson("not json").isEmpty());
}

void LanguagePackManagerTest::verifiesMatchingChecksum() {
    QTemporaryFile file;
    QVERIFY(file.open());
    file.write("model payload");
    file.flush();

    const QString expected = QString::fromLatin1(sha256Of("model payload"));
    QVERIFY(LanguagePackManager::verifyFileIntegrity(file.fileName(), expected));
}

void LanguagePackManagerTest::rejectsMismatchedChecksum() {
    QTemporaryFile file;
    QVERIFY(file.open());
    file.write("model payload");
    file.flush();

    QVERIFY(!LanguagePackManager::verifyFileIntegrity(
        file.fileName(), QStringLiteral("0000000000000000000000000000000000000000000000000000000000000000")));
    QVERIFY(!LanguagePackManager::verifyFileIntegrity(file.fileName(), QString()));
}

void LanguagePackManagerTest::rejectsMissingFile() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath(QStringLiteral("absent.argosmodel"));

    QVERIFY(!LanguagePackManager::verifyFileIntegrity(path, QString::fromLatin1(sha256Of("model payload"))));
}

QTEST_GUILESS_MAIN(LanguagePackManagerTest)

#include "test_language_pack_manager.moc"

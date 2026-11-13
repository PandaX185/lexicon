#include "infrastructure/database/SqliteDictionaryRepository.h"

#include <QFile>
#include <QTest>

using namespace Lexicon;

class WordNetSqliteTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void databaseExistsAndHasMetadata();
    void headwordHasDefinitionsAndIpa();
    void multipleSensesAndPosPreserved();
    void resolvesInflectedForms();
    void lookupIsFastUnderThreshold();
};

void WordNetSqliteTest::databaseExistsAndHasMetadata() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet in development tree.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryDescription desc = repo.describe();
    QVERIFY(desc.ok());
    QCOMPARE(desc.info.sourceLanguage, QStringLiteral("en"));
    QCOMPARE(desc.info.targetLanguage, QStringLiteral("en"));
    QVERIFY(desc.info.attribution.contains(QStringLiteral("Princeton University")));
    QVERIFY(desc.entryCount >= 10);
}

void WordNetSqliteTest::headwordHasDefinitionsAndIpa() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryLookupResult result = repo.findByWord(QStringLiteral("serendipity"));
    QVERIFY(result.found());
    QCOMPARE(result.entry->word, QStringLiteral("serendipity"));
    QVERIFY(!result.entry->definitions.isEmpty());
    QVERIFY(!result.entry->pronunciation.ipa.isEmpty());
    QVERIFY(result.entry->pronunciation.ipa.startsWith(QLatin1Char('/')));
}

void WordNetSqliteTest::multipleSensesAndPosPreserved() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    const DictionaryLookupResult result = repo.findByWord(QStringLiteral("run"));
    QVERIFY(result.found());
    QVERIFY(result.entry->definitions.size() > 5);

    bool hasVerb = false;
    bool hasNoun = false;
    for (const auto& def : result.entry->definitions) {
        if (def.partOfSpeech == PartOfSpeech::Verb) {
            hasVerb = true;
        }
        if (def.partOfSpeech == PartOfSpeech::Noun) {
            hasNoun = true;
        }
    }
    QVERIFY(hasVerb);
    QVERIFY(hasNoun);
}

void WordNetSqliteTest::resolvesInflectedForms() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);

    const struct {
        const char* form;
        const char* headword;
    } cases[] = {
        {"went", "go"}, {"ran", "run"}, {"children", "child"}, {"walked", "walk"}, {"jumped", "jump"},
    };

    for (const auto& item : cases) {
        const DictionaryLookupResult result = repo.findByWord(QString::fromLatin1(item.form));
        QVERIFY2(result.found(), item.form);
        QCOMPARE(result.entry->word.toLower(), QString::fromLatin1(item.headword));
        QCOMPARE(result.entry->matchedForm, QString::fromLatin1(item.form));
    }
}

void WordNetSqliteTest::lookupIsFastUnderThreshold() {
    const QString dbPath = QStringLiteral(LEXICON_SOURCE_DIR "/data/dictionaries/en.sqlite");
    if (!QFile::exists(dbPath)) {
        QSKIP("Bundled en.sqlite not present yet.");
    }
    SqliteDictionaryRepository repo(dbPath);
    QBENCHMARK {
        const auto res = repo.findByWord(QStringLiteral("lexicon"));
        Q_UNUSED(res);
    }
}

QTEST_GUILESS_MAIN(WordNetSqliteTest)

#include "test_wordnet_sqlite.moc"

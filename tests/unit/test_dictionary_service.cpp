#include "dictionary/DictionaryService.h"

#include <QTest>

#include <utility>

using namespace Lexicon;

namespace {

class FakeRepository final : public DictionaryRepository {
  public:
    explicit FakeRepository(DictionaryLookupResult result) : m_result(std::move(result)) {}

    DictionaryLookupResult findByWord(const QString& word) const override {
        lastWord = word;
        return m_result;
    }

    QString location() const override {
        return QStringLiteral("fake.sqlite");
    }

    mutable QString lastWord;
    DictionaryLookupResult m_result;
};

WordEntry makeEntry() {
    WordEntry entry;
    entry.word = QStringLiteral("serendipity");
    Definition definition;
    definition.partOfSpeech = PartOfSpeech::Noun;
    definition.text = QStringLiteral("A fortunate discovery by chance.");
    entry.definitions.append(definition);
    return entry;
}

} // namespace

class DictionaryServiceTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void trimsLookupWord();
    void rejectsEmptyWord();
    void reportsMissingEntry();
    void passesThroughRepositoryFailure();
    void exposesRepositoryLocation();
};

void DictionaryServiceTest::trimsLookupWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  serendipity  "));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("serendipity"));
}

void DictionaryServiceTest::rejectsEmptyWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("   "));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::InvalidArgument);
}

void DictionaryServiceTest::reportsMissingEntry() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("missing"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::WordNotFound);
}

void DictionaryServiceTest::passesThroughRepositoryFailure() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::failure(
        AppError::make(ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open."), QStringLiteral("details"))));
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("serendipity"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseOpenFailed);
    QCOMPARE(result.error.detail, QStringLiteral("details"));
}

void DictionaryServiceTest::exposesRepositoryLocation() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    QCOMPARE(service.repositoryLocation(), QStringLiteral("fake.sqlite"));
}

QTEST_GUILESS_MAIN(DictionaryServiceTest)

#include "test_dictionary_service.moc"

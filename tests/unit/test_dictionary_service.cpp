#include "dictionary/DictionaryService.h"
#include "support/TestDoubles.h"

#include <QTest>

#include <memory>
#include <utility>

using namespace Lexicon;
using namespace Lexicon::Test;

namespace {

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
    void trimsTabsAndNewlines();
    void collapsesInternalWhitespace();
    void stripsTrailingPunctuation();
    void acceptsFourWordPhrase();
    void rejectsSentenceLengthInput();
    void rejectsEmptyWord();
    void rejectsWhitespaceOnlyWord();
    void reportsMissingEntry();
    void notFoundMessageContainsWord();
    void passesThroughRepositoryFailure();
    void passesThroughFailureWithoutDetail();
    void handlesNullRepository();
    void exposesRepositoryLocation();
    void handlesNullRepositoryLocation();
    void passesThroughUnicodeWord();
    void passesThroughVeryLongWord();
    void cachesSuccessfulLookups();
    void doesNotCacheMisses();
    void clearCacheForcesNewLookup();
    void disablesCacheWhenSizeIsZero();
    void negativeCacheSizeDisablesCache();
    void cacheReturnsFirstResultForRepeatedLookup();
    void notFoundMessageQuotesWordWithoutBackslashes();
};

void DictionaryServiceTest::trimsLookupWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  serendipity  "));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("serendipity"));
    QCOMPARE(repositoryPtr->calls, 1);
}

void DictionaryServiceTest::trimsTabsAndNewlines() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("\t\n serendipity \r\n"));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("serendipity"));
}

void DictionaryServiceTest::collapsesInternalWhitespace() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  ice   cream  "));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("ice cream"));
}

void DictionaryServiceTest::stripsTrailingPunctuation() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  \"serendipity\",  "));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("serendipity"));
}

void DictionaryServiceTest::acceptsFourWordPhrase() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("one two three four"));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("one two three four"));
}

void DictionaryServiceTest::rejectsSentenceLengthInput() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("one two three four five"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::InvalidArgument);
    QCOMPARE(repositoryPtr->calls, 0);
}

void DictionaryServiceTest::rejectsEmptyWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QString());

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::InvalidArgument);
    QCOMPARE(repositoryPtr->calls, 0);
}

void DictionaryServiceTest::rejectsWhitespaceOnlyWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral(" \t\n\r "));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::InvalidArgument);
    QCOMPARE(repositoryPtr->calls, 0);
}

void DictionaryServiceTest::reportsMissingEntry() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("missing"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::WordNotFound);
}

void DictionaryServiceTest::notFoundMessageContainsWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  missing  "));

    QVERIFY(result.error.message.contains(QStringLiteral("missing")));
    QCOMPARE(result.error.detail, QStringLiteral("missing"));
}

void DictionaryServiceTest::passesThroughRepositoryFailure() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::failure(
        AppError::make(ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open."), QStringLiteral("details"))));
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("serendipity"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseOpenFailed);
    QCOMPARE(result.error.message, QStringLiteral("Could not open."));
    QCOMPARE(result.error.detail, QStringLiteral("details"));
}

void DictionaryServiceTest::passesThroughFailureWithoutDetail() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::failure(
        AppError::make(ErrorCode::DatabaseQueryFailed, QStringLiteral("Query failed."))));
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("serendipity"));

    QCOMPARE(result.error.code, ErrorCode::DatabaseQueryFailed);
    QVERIFY(result.error.detail.isEmpty());
}

void DictionaryServiceTest::handlesNullRepository() {
    DictionaryService service(nullptr);

    const DictionaryLookupResult result = service.lookup(QStringLiteral("serendipity"));

    QVERIFY(!result.found());
    QCOMPARE(result.error.code, ErrorCode::DatabaseUnavailable);
}

void DictionaryServiceTest::exposesRepositoryLocation() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    QCOMPARE(service.repositoryLocation(), QStringLiteral("fake.sqlite"));
}

void DictionaryServiceTest::handlesNullRepositoryLocation() {
    DictionaryService service(nullptr);

    QVERIFY(service.repositoryLocation().isEmpty());
}

void DictionaryServiceTest::passesThroughUnicodeWord() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("  café  "));

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("café"));
}

void DictionaryServiceTest::passesThroughVeryLongWord() {
    const QString word(4096, QLatin1Char('a'));
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(word);

    QVERIFY(result.found());
    QCOMPARE(repositoryPtr->lastWord, word);
}

void DictionaryServiceTest::cachesSuccessfulLookups() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult first = service.lookup(QStringLiteral("serendipity"));
    const DictionaryLookupResult second = service.lookup(QStringLiteral("serendipity"));

    QVERIFY(first.found());
    QVERIFY(second.found());
    QCOMPARE(repositoryPtr->calls, 1);
}

void DictionaryServiceTest::doesNotCacheMisses() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    QVERIFY(!service.lookup(QStringLiteral("missing")).found());
    QVERIFY(!service.lookup(QStringLiteral("missing")).found());

    QCOMPARE(repositoryPtr->calls, 2);
}

void DictionaryServiceTest::clearCacheForcesNewLookup() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());
    service.clearCache();
    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());

    QCOMPARE(repositoryPtr->calls, 2);
}

void DictionaryServiceTest::disablesCacheWhenSizeIsZero() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository), 0);

    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());
    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());

    QCOMPARE(repositoryPtr->calls, 2);
}

void DictionaryServiceTest::negativeCacheSizeDisablesCache() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository), -10);

    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());
    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());

    QCOMPARE(repositoryPtr->calls, 2);
}

void DictionaryServiceTest::cacheReturnsFirstResultForRepeatedLookup() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::success(makeEntry()));
    FakeRepository* repositoryPtr = repository.get();
    DictionaryService service(std::move(repository));

    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());
    repositoryPtr->result = DictionaryLookupResult::notFound();

    QVERIFY(service.lookup(QStringLiteral("serendipity")).found());
    QCOMPARE(repositoryPtr->calls, 1);
}

void DictionaryServiceTest::notFoundMessageQuotesWordWithoutBackslashes() {
    auto repository = std::make_unique<FakeRepository>(DictionaryLookupResult::notFound());
    DictionaryService service(std::move(repository));

    const DictionaryLookupResult result = service.lookup(QStringLiteral("missing"));

    QCOMPARE(result.error.message, QStringLiteral("No entry found for \"missing\"."));
}

QTEST_GUILESS_MAIN(DictionaryServiceTest)

#include "test_dictionary_service.moc"

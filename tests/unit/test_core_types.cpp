#include "core/errors/AppError.h"
#include "core/types/Result.h"
#include "core/types/SelectionResult.h"
#include "core/types/ServiceStatus.h"
#include "dictionary/models/WordEntry.h"

#include <QTest>

using namespace Lexicon;

class CoreTypesTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void resultHoldsValue();
    void resultHoldsError();
    void selectionRequiresNonWhitespaceText();
    void serviceStatusFactories();
    void appErrorFactories();
    void errorCodeNames_data();
    void errorCodeNames();
    void wordEntryValidity();
    void wordEntryNormalization();
    void partOfSpeechRoundTrip_data();
    void partOfSpeechRoundTrip();
    void partOfSpeechUnknownValue();
};

void CoreTypesTest::resultHoldsValue() {
    struct Value {
        int number = 0;
    };

    const Result<Value> result(Value{.number = 42});

    QVERIFY(result.hasValue());
    QVERIFY(static_cast<bool>(result));
    QCOMPARE(result.value().number, 42);
    QCOMPARE(result->number, 42);
}

void CoreTypesTest::resultHoldsError() {
    const Result<int> result(
        AppError::make(ErrorCode::InvalidArgument, QStringLiteral("bad"), QStringLiteral("detail")));

    QVERIFY(!result.hasValue());
    QVERIFY(!static_cast<bool>(result));
    QCOMPARE(result.error().code, ErrorCode::InvalidArgument);
    QCOMPARE(result.error().message, QStringLiteral("bad"));
    QCOMPARE(result.error().detail, QStringLiteral("detail"));
}

void CoreTypesTest::selectionRequiresNonWhitespaceText() {
    const SelectionResult success = SelectionResult::success(QStringLiteral("serendipity"));
    QVERIFY(success.hasSelection());

    const SelectionResult whitespace = SelectionResult::success(QStringLiteral(" \t\n "));
    QVERIFY(!whitespace.hasSelection());

    const SelectionResult failure =
        SelectionResult::failure(AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("no")));
    QVERIFY(!failure.hasSelection());
    QCOMPARE(failure.error.code, ErrorCode::SelectionUnavailable);
}

void CoreTypesTest::serviceStatusFactories() {
    const ServiceStatus success = ServiceStatus::success();
    QVERIFY(success.available);
    QVERIFY(!success.error.isError());

    const ServiceStatus failure =
        ServiceStatus::failure(AppError::make(ErrorCode::TrayUnavailable, QStringLiteral("no tray")));
    QVERIFY(!failure.available);
    QCOMPARE(failure.error.code, ErrorCode::TrayUnavailable);
}

void CoreTypesTest::appErrorFactories() {
    const AppError none = AppError::none();
    QVERIFY(!none.isError());
    QVERIFY(!static_cast<bool>(none));

    const AppError error = AppError::make(ErrorCode::WordNotFound, QStringLiteral("missing"));
    QVERIFY(error.isError());
    QVERIFY(static_cast<bool>(error));
    QCOMPARE(error.code, ErrorCode::WordNotFound);
    QVERIFY(error.detail.isEmpty());
}

void CoreTypesTest::errorCodeNames_data() {
    QTest::addColumn<ErrorCode>("code");
    QTest::addColumn<QString>("name");

    QTest::newRow("none") << ErrorCode::None << QStringLiteral("none");
    QTest::newRow("invalid") << ErrorCode::InvalidArgument << QStringLiteral("invalid_argument");
    QTest::newRow("config-parse") << ErrorCode::ConfigParseError << QStringLiteral("config_parse_error");
    QTest::newRow("config-validation") << ErrorCode::ConfigValidationError << QStringLiteral("config_validation_error");
    QTest::newRow("db-unavailable") << ErrorCode::DatabaseUnavailable << QStringLiteral("database_unavailable");
    QTest::newRow("db-open") << ErrorCode::DatabaseOpenFailed << QStringLiteral("database_open_failed");
    QTest::newRow("db-query") << ErrorCode::DatabaseQueryFailed << QStringLiteral("database_query_failed");
    QTest::newRow("word-not-found") << ErrorCode::WordNotFound << QStringLiteral("word_not_found");
    QTest::newRow("selection") << ErrorCode::SelectionUnavailable << QStringLiteral("selection_unavailable");
    QTest::newRow("selection-unsupported")
        << ErrorCode::SelectionUnsupported << QStringLiteral("selection_unsupported");
    QTest::newRow("selection-not-text") << ErrorCode::SelectionNotText << QStringLiteral("selection_not_text");
    QTest::newRow("selection-too-long") << ErrorCode::SelectionTooLong << QStringLiteral("selection_too_long");
    QTest::newRow("shortcut") << ErrorCode::ShortcutRegistrationFailed
                              << QStringLiteral("shortcut_registration_failed");
    QTest::newRow("tray") << ErrorCode::TrayUnavailable << QStringLiteral("tray_unavailable");
    QTest::newRow("unknown") << ErrorCode::Unknown << QStringLiteral("unknown");
}

void CoreTypesTest::errorCodeNames() {
    QFETCH(ErrorCode, code);
    QFETCH(QString, name);

    QCOMPARE(errorCodeToString(code), name);
}

void CoreTypesTest::wordEntryValidity() {
    WordEntry entry;
    QVERIFY(!entry.isValid());

    entry.word = QStringLiteral("  serendipity  ");
    QVERIFY(!entry.isValid());

    entry.definitions.append(
        Definition{.text = QStringLiteral("A fortunate discovery."), .partOfSpeech = PartOfSpeech::Other, .label = {}});
    QVERIFY(entry.isValid());
}

void CoreTypesTest::wordEntryNormalization() {
    WordEntry entry;
    entry.word = QStringLiteral("  SeReNdIpItY  ");

    QCOMPARE(entry.normalizedWord(), QStringLiteral("serendipity"));
}

void CoreTypesTest::partOfSpeechRoundTrip_data() {
    QTest::addColumn<PartOfSpeech>("value");
    QTest::addColumn<QString>("text");

    QTest::newRow("noun") << PartOfSpeech::Noun << QStringLiteral("noun");
    QTest::newRow("verb") << PartOfSpeech::Verb << QStringLiteral("verb");
    QTest::newRow("adjective") << PartOfSpeech::Adjective << QStringLiteral("adjective");
    QTest::newRow("adverb") << PartOfSpeech::Adverb << QStringLiteral("adverb");
    QTest::newRow("pronoun") << PartOfSpeech::Pronoun << QStringLiteral("pronoun");
    QTest::newRow("preposition") << PartOfSpeech::Preposition << QStringLiteral("preposition");
    QTest::newRow("conjunction") << PartOfSpeech::Conjunction << QStringLiteral("conjunction");
    QTest::newRow("determiner") << PartOfSpeech::Determiner << QStringLiteral("determiner");
    QTest::newRow("interjection") << PartOfSpeech::Interjection << QStringLiteral("interjection");
    QTest::newRow("other") << PartOfSpeech::Other << QStringLiteral("other");
}

void CoreTypesTest::partOfSpeechRoundTrip() {
    QFETCH(PartOfSpeech, value);
    QFETCH(QString, text);

    QCOMPARE(partOfSpeechToString(value), text);
    QCOMPARE(partOfSpeechFromString(text), value);
    QCOMPARE(partOfSpeechFromString(QStringLiteral("  ") + text.toUpper() + QStringLiteral("  ")), value);
}

void CoreTypesTest::partOfSpeechUnknownValue() {
    QCOMPARE(partOfSpeechFromString(QStringLiteral("prepositional phrase")), PartOfSpeech::Other);
    QCOMPARE(partOfSpeechFromString(QString()), PartOfSpeech::Other);
}

QTEST_GUILESS_MAIN(CoreTypesTest)

#include "test_core_types.moc"

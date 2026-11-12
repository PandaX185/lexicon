#include "core/interfaces/PrimarySelectionSource.h"
#include "core/selection/PrimarySelectionProvider.h"
#include "core/selection/SelectionTextPolicy.h"
#include "core/selection/UnsupportedPrimarySelectionSource.h"

#include <QStringList>
#include <QTest>

#include <memory>

using namespace Lexicon;

class FakePrimarySelectionSource final : public PrimarySelectionSource {
  public:
    PrimarySelectionReadResult readPrimarySelection(const QStringList& mimeTypes) override {
        ++calls;
        lastMimeTypes = mimeTypes;
        return result;
    }

    [[nodiscard]] QString displayName() const override {
        return QStringLiteral("fake primary selection");
    }

    PrimarySelectionReadResult result = PrimarySelectionReadResult::failure(
        AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("Nothing selected.")));
    QStringList lastMimeTypes;
    int calls = 0;
};

class SelectionProviderTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void policyUsesPlainTextPreference();
    void passesAcceptedTypesToSource();
    void trimsSelectionText();
    void propagatesSourceErrors_data();
    void propagatesSourceErrors();
    void reportsEmptySelection();
    void reportsUnexpectedMimeType();
    void reportsInvalidUtf8();
    void rejectsSelectionsOverLimit_data();
    void rejectsSelectionsOverLimit();
    void acceptsSelectionAtLimit();
    void delegatesDisplayName();
    void unsupportedSourceFailsExplicitly();
};

void SelectionProviderTest::policyUsesPlainTextPreference() {
    const SelectionTextPolicy policy;

    const QStringList expectedMimeTypes{QStringLiteral("text/plain;charset=utf-8"), QStringLiteral("text/plain"),
                                        QStringLiteral("utf8_string")};

    QCOMPARE(policy.maxCharacters(), 128);
    QCOMPARE(policy.acceptedMimeTypes(), expectedMimeTypes);
}

void SelectionProviderTest::passesAcceptedTypesToSource() {
    auto source = std::make_unique<FakePrimarySelectionSource>();
    FakePrimarySelectionSource* sourcePointer = source.get();
    source->result = PrimarySelectionReadResult::success(QStringLiteral("text/plain"), QByteArray("word"));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(result.hasSelection());
    QCOMPARE(*result.text, QStringLiteral("word"));
    QCOMPARE(sourcePointer->lastMimeTypes, SelectionTextPolicy{}.acceptedMimeTypes());
}

void SelectionProviderTest::trimsSelectionText() {
    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result =
        PrimarySelectionReadResult::success(QStringLiteral("text/plain;charset=utf-8"), QByteArray("  serendipity \n"));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(result.hasSelection());
    QCOMPARE(*result.text, QStringLiteral("serendipity"));
}

void SelectionProviderTest::propagatesSourceErrors_data() {
    QTest::addColumn<ErrorCode>("code");
    QTest::addColumn<QString>("message");

    QTest::newRow("unavailable") << ErrorCode::SelectionUnavailable << QStringLiteral("Nothing selected.");
    QTest::newRow("unsupported") << ErrorCode::SelectionUnsupported << QStringLiteral("Protocol missing.");
}

void SelectionProviderTest::propagatesSourceErrors() {
    QFETCH(ErrorCode, code);
    QFETCH(QString, message);

    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result =
        PrimarySelectionReadResult::failure(AppError::make(code, message, QStringLiteral("transport detail")));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, code);
    QCOMPARE(result.error.message, message);
    QCOMPARE(result.error.detail, QStringLiteral("transport detail"));
}

void SelectionProviderTest::reportsEmptySelection() {
    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result = PrimarySelectionReadResult::success(QStringLiteral("text/plain"), QByteArray(" \t\n "));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnavailable);
}

void SelectionProviderTest::reportsUnexpectedMimeType() {
    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result =
        PrimarySelectionReadResult::success(QStringLiteral("application/octet-stream"), QByteArray("binary"));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionNotText);
}

void SelectionProviderTest::reportsInvalidUtf8() {
    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result = PrimarySelectionReadResult::success(QStringLiteral("text/plain"), QByteArray(1, '\xFF'));
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionNotText);
    QVERIFY(result.text == std::nullopt);
}

void SelectionProviderTest::rejectsSelectionsOverLimit_data() {
    QTest::addColumn<int>("limit");
    QTest::addColumn<int>("length");

    QTest::newRow("default-limit") << 128 << 129;
    QTest::newRow("custom-limit") << 4 << 5;
}

void SelectionProviderTest::rejectsSelectionsOverLimit() {
    QFETCH(int, limit);
    QFETCH(int, length);

    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result =
        PrimarySelectionReadResult::success(QStringLiteral("text/plain"), QString(length, QLatin1Char('a')).toUtf8());
    PrimarySelectionProvider provider(std::move(source), SelectionTextPolicy(limit));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionTooLong);
}

void SelectionProviderTest::acceptsSelectionAtLimit() {
    const QString text(128, QLatin1Char('a'));
    auto source = std::make_unique<FakePrimarySelectionSource>();
    source->result = PrimarySelectionReadResult::success(QStringLiteral("text/plain"), text.toUtf8());
    PrimarySelectionProvider provider(std::move(source));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(result.hasSelection());
    QCOMPARE(*result.text, text);
}

void SelectionProviderTest::delegatesDisplayName() {
    PrimarySelectionProvider provider(std::make_unique<FakePrimarySelectionSource>());

    QCOMPARE(provider.displayName(), QStringLiteral("fake primary selection"));
}

void SelectionProviderTest::unsupportedSourceFailsExplicitly() {
    const QString reason = QStringLiteral("X11 is not wired up yet.");
    PrimarySelectionProvider provider(std::make_unique<UnsupportedPrimarySelectionSource>(
        QStringLiteral("x11 selection"), QStringLiteral("Selection retrieval is unsupported."), reason));

    const SelectionResult result = provider.currentSelection();

    QVERIFY(!result.hasSelection());
    QCOMPARE(result.error.code, ErrorCode::SelectionUnsupported);
    QCOMPARE(result.error.detail, reason);
    QCOMPARE(provider.displayName(), QStringLiteral("x11 selection"));
}

QTEST_GUILESS_MAIN(SelectionProviderTest)

#include "test_selection_provider.moc"

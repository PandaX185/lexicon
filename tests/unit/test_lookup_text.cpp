#include "core/text/LookupText.h"

#include <QTest>

using namespace Lexicon;

class LookupTextTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void trimsAndCollapsesWhitespace();
    void stripsTrailingSentencePunctuation();
    void stripsWrappingQuotesAndBrackets();
    void keepsAbbreviationDots();
    void stripsRepeatedSentenceDots();
    void keepsInternalApostrophesAndHyphens();
    void stripsUnmatchedClosingQuote();
    void handlesEmptyInput();
    void countsWords();
};

void LookupTextTest::trimsAndCollapsesWhitespace() {
    QCOMPARE(normalizeLookupText(QStringLiteral("  ice   cream \t\n")), QStringLiteral("ice cream"));
}

void LookupTextTest::stripsTrailingSentencePunctuation() {
    QCOMPARE(normalizeLookupText(QStringLiteral("serendipity,")), QStringLiteral("serendipity"));
    QCOMPARE(normalizeLookupText(QStringLiteral("serendipity.")), QStringLiteral("serendipity"));
    QCOMPARE(normalizeLookupText(QStringLiteral("word?!")), QStringLiteral("word"));
}

void LookupTextTest::stripsWrappingQuotesAndBrackets() {
    QCOMPARE(normalizeLookupText(QStringLiteral("\"serendipity\"")), QStringLiteral("serendipity"));
    QCOMPARE(normalizeLookupText(QStringLiteral("(serendipity)")), QStringLiteral("serendipity"));
    QCOMPARE(normalizeLookupText(QStringLiteral("\u201Cserendipity\u201D")), QStringLiteral("serendipity"));
    QCOMPARE(normalizeLookupText(QStringLiteral("(\"serendipity\").")), QStringLiteral("serendipity"));
}

void LookupTextTest::keepsAbbreviationDots() {
    QCOMPARE(normalizeLookupText(QStringLiteral("U.S.")), QStringLiteral("U.S."));
    QCOMPARE(normalizeLookupText(QStringLiteral("e.g.")), QStringLiteral("e.g."));
}

void LookupTextTest::stripsRepeatedSentenceDots() {
    QCOMPARE(normalizeLookupText(QStringLiteral("serendipity...")), QStringLiteral("serendipity"));
}

void LookupTextTest::keepsInternalApostrophesAndHyphens() {
    QCOMPARE(normalizeLookupText(QStringLiteral("o'clock")), QStringLiteral("o'clock"));
    QCOMPARE(normalizeLookupText(QStringLiteral("well-being,")), QStringLiteral("well-being"));
}

void LookupTextTest::stripsUnmatchedClosingQuote() {
    QCOMPARE(normalizeLookupText(QStringLiteral("serendipity\"")), QStringLiteral("serendipity"));
}

void LookupTextTest::handlesEmptyInput() {
    QCOMPARE(normalizeLookupText(QString()), QString());
    QCOMPARE(normalizeLookupText(QStringLiteral("  \t\n ")), QString());
    QCOMPARE(normalizeLookupText(QStringLiteral("\"\"")), QString());
}

void LookupTextTest::countsWords() {
    QCOMPARE(countLookupWords(QString()), 0);
    QCOMPARE(countLookupWords(QStringLiteral("serendipity")), 1);
    QCOMPARE(countLookupWords(QStringLiteral("past participle")), 2);
    QCOMPARE(countLookupWords(QStringLiteral("one two three four")), 4);
    QCOMPARE(countLookupWords(QStringLiteral("one two three four five")), 5);
}

QTEST_GUILESS_MAIN(LookupTextTest)

#include "test_lookup_text.moc"

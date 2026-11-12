#include "dictionary/ArpabetToIpa.h"

#include <QTest>

using namespace Lexicon;

class ArpabetToIpaTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void convertsSingleWord();
    void handlesPrimaryAndSecondaryStress();
    void handlesUnstressedVowels();
    void handlesConsonantsAndAffricates();
    void handlesMultiplePronunciationVariants();
    void handlesInvalidInputGracefully();
};

void ArpabetToIpaTest::convertsSingleWord() {
    const QString arpabet = QStringLiteral("S EH2 R AH0 N D IH1 P IH0 T IY0");
    const QString ipa = arpabetToIpa(arpabet);
    QCOMPARE(ipa, QStringLiteral("/ˌsɛrənˈdɪpɪti/"));
}

void ArpabetToIpaTest::handlesPrimaryAndSecondaryStress() {
    const QString arpabet = QStringLiteral("L EH1 K S IH0 K AA2 N");
    const QString ipa = arpabetToIpa(arpabet);
    QCOMPARE(ipa, QStringLiteral("/ˈlɛksɪˌkɑn/"));
}

void ArpabetToIpaTest::handlesUnstressedVowels() {
    QCOMPARE(arpabetToIpa(QStringLiteral("AE1 P AH0 L")), QStringLiteral("/ˈæpəl/"));
}

void ArpabetToIpaTest::handlesConsonantsAndAffricates() {
    QCOMPARE(arpabetToIpa(QStringLiteral("CH ER1 CH")), QStringLiteral("/ˈtʃɜːrtʃ/"));
    QCOMPARE(arpabetToIpa(QStringLiteral("JH AH1 JH")), QStringLiteral("/ˈdʒʌdʒ/"));
    QCOMPARE(arpabetToIpa(QStringLiteral("DH AH0")), QStringLiteral("/ðə/"));
    QCOMPARE(arpabetToIpa(QStringLiteral("TH IH1 NG K")), QStringLiteral("/ˈθɪŋk/"));
}

void ArpabetToIpaTest::handlesMultiplePronunciationVariants() {
    const QString ipa = arpabetToIpa(QStringLiteral("R EH1 D ; R IY1 D"));
    QCOMPARE(ipa, QStringLiteral("/ˈrɛd/, /ˈriːd/"));
}

void ArpabetToIpaTest::handlesInvalidInputGracefully() {
    QCOMPARE(arpabetToIpa(QStringLiteral("")), QStringLiteral(""));
    QCOMPARE(arpabetToIpa(QStringLiteral("   ")), QStringLiteral(""));
}

QTEST_GUILESS_MAIN(ArpabetToIpaTest)

#include "test_arpabet_to_ipa.moc"

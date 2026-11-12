#include "dictionary/ArpabetToIpa.h"

#include <QHash>
#include <QRegularExpression>
#include <QSet>
#include <QStringList>

namespace Lexicon {
namespace {

const QHash<QString, QString>& phonemeMap() {
    static const QHash<QString, QString> map = {
        {QStringLiteral("AA"), QStringLiteral("ɑ")},  {QStringLiteral("AE"), QStringLiteral("æ")},
        {QStringLiteral("AH"), QStringLiteral("ʌ")},  {QStringLiteral("AO"), QStringLiteral("ɔ")},
        {QStringLiteral("AW"), QStringLiteral("aʊ")}, {QStringLiteral("AY"), QStringLiteral("aɪ")},
        {QStringLiteral("B"), QStringLiteral("b")},   {QStringLiteral("CH"), QStringLiteral("tʃ")},
        {QStringLiteral("D"), QStringLiteral("d")},   {QStringLiteral("DH"), QStringLiteral("ð")},
        {QStringLiteral("EH"), QStringLiteral("ɛ")},  {QStringLiteral("ER"), QStringLiteral("ɜːr")},
        {QStringLiteral("EY"), QStringLiteral("eɪ")}, {QStringLiteral("F"), QStringLiteral("f")},
        {QStringLiteral("G"), QStringLiteral("ɡ")},   {QStringLiteral("HH"), QStringLiteral("h")},
        {QStringLiteral("IH"), QStringLiteral("ɪ")},  {QStringLiteral("IY"), QStringLiteral("iː")},
        {QStringLiteral("JH"), QStringLiteral("dʒ")}, {QStringLiteral("K"), QStringLiteral("k")},
        {QStringLiteral("L"), QStringLiteral("l")},   {QStringLiteral("M"), QStringLiteral("m")},
        {QStringLiteral("N"), QStringLiteral("n")},   {QStringLiteral("NG"), QStringLiteral("ŋ")},
        {QStringLiteral("OW"), QStringLiteral("oʊ")}, {QStringLiteral("OY"), QStringLiteral("ɔɪ")},
        {QStringLiteral("P"), QStringLiteral("p")},   {QStringLiteral("R"), QStringLiteral("r")},
        {QStringLiteral("S"), QStringLiteral("s")},   {QStringLiteral("SH"), QStringLiteral("ʃ")},
        {QStringLiteral("T"), QStringLiteral("t")},   {QStringLiteral("TH"), QStringLiteral("θ")},
        {QStringLiteral("UH"), QStringLiteral("ʊ")},  {QStringLiteral("UW"), QStringLiteral("uː")},
        {QStringLiteral("V"), QStringLiteral("v")},   {QStringLiteral("W"), QStringLiteral("w")},
        {QStringLiteral("Y"), QStringLiteral("j")},   {QStringLiteral("Z"), QStringLiteral("z")},
        {QStringLiteral("ZH"), QStringLiteral("ʒ")},
    };
    return map;
}

const QSet<QString>& vowels() {
    static const QSet<QString> set = {
        QStringLiteral("AA"), QStringLiteral("AE"), QStringLiteral("AH"), QStringLiteral("AO"), QStringLiteral("AW"),
        QStringLiteral("AY"), QStringLiteral("EH"), QStringLiteral("ER"), QStringLiteral("EY"), QStringLiteral("IH"),
        QStringLiteral("IY"), QStringLiteral("OW"), QStringLiteral("OY"), QStringLiteral("UH"), QStringLiteral("UW"),
    };
    return set;
}

bool isValidOnsetPair(const QString& c1, const QString& c2) {
    static const QSet<QString> validPairs = {
        QStringLiteral("P R"),  QStringLiteral("P L"),  QStringLiteral("B R"), QStringLiteral("B L"),
        QStringLiteral("T R"),  QStringLiteral("T W"),  QStringLiteral("D R"), QStringLiteral("D W"),
        QStringLiteral("K R"),  QStringLiteral("K L"),  QStringLiteral("K W"), QStringLiteral("G R"),
        QStringLiteral("G L"),  QStringLiteral("G W"),  QStringLiteral("F R"), QStringLiteral("F L"),
        QStringLiteral("TH R"), QStringLiteral("TH W"), QStringLiteral("S P"), QStringLiteral("S T"),
        QStringLiteral("S K"),  QStringLiteral("S M"),  QStringLiteral("S N"), QStringLiteral("S L"),
        QStringLiteral("S W"),  QStringLiteral("SH R"),
    };
    return validPairs.contains(c1 + QLatin1Char(' ') + c2);
}

void splitCodaAndOnset(const QStringList& consonants, bool hasPrecedingVowel, QStringList& coda, QStringList& onset) {
    if (!hasPrecedingVowel) {
        coda.clear();
        onset = consonants;
        return;
    }

    if (consonants.size() <= 1) {
        coda.clear();
        onset = consonants;
        return;
    }

    if (consonants.size() == 2) {
        if (isValidOnsetPair(consonants.at(0), consonants.at(1))) {
            coda.clear();
            onset = consonants;
        } else {
            coda = {consonants.at(0)};
            onset = {consonants.at(1)};
        }
        return;
    }

    // 3 or more consonants: check if last two form an onset pair
    const auto lastIdx = consonants.size() - 1;
    if (isValidOnsetPair(consonants.at(lastIdx - 1), consonants.at(lastIdx))) {
        coda = consonants.mid(0, lastIdx - 1);
        onset = {consonants.at(lastIdx - 1), consonants.at(lastIdx)};
    } else {
        coda = consonants.mid(0, lastIdx);
        onset = {consonants.at(lastIdx)};
    }
}

QString convertSingleVariant(const QStringList& tokens) {
    if (tokens.isEmpty()) {
        return {};
    }
    const auto& map = phonemeMap();
    const auto& vowelSet = vowels();
    static const QRegularExpression regex(QStringLiteral("^([A-Z]+)([0-2])?$"));

    QString result;
    QStringList currentConsonants;
    bool hasVowel = false;

    for (const QString& token : tokens) {
        const QRegularExpressionMatch match = regex.match(token);
        if (!match.hasMatch()) {
            continue;
        }
        const QString phoneme = match.captured(1);
        const QString stress = match.captured(2);

        if (vowelSet.contains(phoneme)) {
            QString vowelIpa = map.value(phoneme, phoneme);
            if (phoneme == QStringLiteral("AH") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("ə");
            } else if (phoneme == QStringLiteral("ER") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("ər");
            } else if (phoneme == QStringLiteral("IY") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("i");
            } else if (phoneme == QStringLiteral("UW") && stress == QStringLiteral("0")) {
                vowelIpa = QStringLiteral("u");
            }

            QStringList coda;
            QStringList onset;
            splitCodaAndOnset(currentConsonants, hasVowel, coda, onset);

            for (const QString& c : coda) {
                result += map.value(c, c);
            }

            if (stress == QStringLiteral("1")) {
                result += QStringLiteral("ˈ");
            } else if (stress == QStringLiteral("2")) {
                result += QStringLiteral("ˌ");
            }

            for (const QString& o : onset) {
                result += map.value(o, o);
            }

            result += vowelIpa;
            currentConsonants.clear();
            hasVowel = true;
        } else {
            currentConsonants.append(phoneme);
        }
    }

    for (const QString& consonant : currentConsonants) {
        result += map.value(consonant, consonant);
    }

    return QStringLiteral("/%1/").arg(result);
}

} // namespace

QString arpabetToIpa(const QString& arpabet) {
    const QString trimmed = arpabet.trimmed();
    if (trimmed.isEmpty()) {
        return {};
    }

    const QStringList rawVariants = trimmed.split(QLatin1Char(';'), Qt::SkipEmptyParts);
    QStringList convertedVariants;
    for (const QString& rawVariant : rawVariants) {
        const QStringList tokens =
            rawVariant.trimmed().split(QRegularExpression(QStringLiteral("\\s+")), Qt::SkipEmptyParts);
        const QString converted = convertSingleVariant(tokens);
        if (!converted.isEmpty()) {
            convertedVariants.append(converted);
        }
    }

    return convertedVariants.join(QStringLiteral(", "));
}

} // namespace Lexicon

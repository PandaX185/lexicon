#include "infrastructure/translation/ArgosTranslationService.h"

#include <QFileInfo>
#include <QStandardPaths>
#include <QTest>

using namespace Lexicon;

namespace {

bool workerAvailable() {
    return !QStandardPaths::findExecutable(QStringLiteral("python3")).isEmpty() &&
           QFileInfo::exists(QStringLiteral(LEXICON_SOURCE_DIR "/scripts/argos_worker.py"));
}

// The worker script is installed for production builds; tests point at the
// working tree copy instead of inventing an installation prefix.
std::unique_ptr<ArgosProcessWorker> developmentWorker() {
    return std::make_unique<ArgosProcessWorker>(QStringLiteral("python3"),
                                                QStringList{
                                                    QStringLiteral(LEXICON_SOURCE_DIR "/scripts/"
                                                                                      "argos_worker.py"),
                                                });
}

} // namespace

class TranslationServiceTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void rejectsEmptyInput();
    void reportsNoInstalledPairsWithoutArgos();
    void reportsMissingPairAsUninstalled();
};

void TranslationServiceTest::rejectsEmptyInput() {
    ArgosTranslationService service;
    const Result<QString> result = service.translate(QStringLiteral("   "), QStringLiteral("en"), QStringLiteral("es"));

    QVERIFY(!result.hasValue());
    QCOMPARE(result.error().code, ErrorCode::InvalidArgument);
}

void TranslationServiceTest::reportsNoInstalledPairsWithoutArgos() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosTranslationService service(developmentWorker());
    const Result<QList<LanguagePair>> pairs = service.installedPairs();

    QVERIFY(pairs.hasValue());
    QVERIFY(!service.isPairInstalled(QStringLiteral("en"), QStringLiteral("fr")));
}

void TranslationServiceTest::reportsMissingPairAsUninstalled() {
    if (!workerAvailable()) {
        QSKIP("python3 or the Argos worker script is not available.");
    }

    ArgosTranslationService service(developmentWorker());
    const Result<QString> result =
        service.translate(QStringLiteral("hello"), QStringLiteral("en"), QStringLiteral("zz"));

    QVERIFY(!result.hasValue());
    QCOMPARE(result.error().code, ErrorCode::TranslationUnavailable);
}

QTEST_GUILESS_MAIN(TranslationServiceTest)

#include "test_translation_service.moc"

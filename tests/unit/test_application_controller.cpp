#include "app/ApplicationController.h"
#include "config/Config.h"
#include "dictionary/DictionaryService.h"
#include "dictionary/models/WordEntry.h"
#include "support/TestDoubles.h"

#include <QTest>

#include <memory>
#include <utility>

using namespace Lexicon;
using namespace Lexicon::Test;

namespace {

WordEntry makeEntry(QString word = QStringLiteral("serendipity"), int definitionCount = 1) {
    WordEntry entry;
    entry.word = std::move(word);
    for (int index = 0; index < definitionCount; ++index) {
        Definition definition;
        definition.partOfSpeech = PartOfSpeech::Noun;
        definition.text = QStringLiteral("Definition %1").arg(index + 1);
        entry.definitions.append(definition);
    }
    return entry;
}

// The controller copies its configuration, so tests that need a translated
// lookup have to set the language pair before the harness exists.
Config translatedConfig() {
    Config config;
    config.dictionary.sourceLanguage = QStringLiteral("en");
    config.dictionary.targetLanguage = QStringLiteral("de");
    return config;
}

struct Harness {
    explicit Harness(Config initialConfig = Config{}) : config(std::move(initialConfig)) {
        auto repository = std::make_unique<FakeRepository>();
        repositoryPtr = repository.get();
        auto dictionary = std::make_unique<DictionaryService>(std::move(repository));

        auto selectionProvider = std::make_unique<FakeSelectionProvider>();
        selectionPtr = selectionProvider.get();
        auto shortcutService = std::make_unique<FakeShortcutService>();
        shortcutsPtr = shortcutService.get();
        auto trayService = std::make_unique<FakeTrayService>();
        trayPtr = trayService.get();

        auto translationService = std::make_unique<FakeTranslationService>();
        translationPtr = translationService.get();

        controller = std::make_unique<ApplicationController>(
            config, std::move(dictionary), std::move(translationService), std::move(selectionProvider),
            std::move(shortcutService), std::move(trayService), presenter);
    }

    Config config;
    RecordingPresenter presenter;
    FakeRepository* repositoryPtr = nullptr;
    FakeTranslationService* translationPtr = nullptr;
    FakeSelectionProvider* selectionPtr = nullptr;
    FakeShortcutService* shortcutsPtr = nullptr;
    FakeTrayService* trayPtr = nullptr;
    std::unique_ptr<ApplicationController> controller;
};

} // namespace

class ApplicationControllerTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void startRegistersShortcutAndTray();
    void shortcutCallbackLooksUpSelection();
    void lookupWordShowsEntry();
    void lookupWordRejectsEmptyInput();
    void lookupWordReportsMissingEntry();
    void lookupWordReportsRepositoryFailure();
    void lookupSelectionReportsUnavailable();
    void lookupSelectionIncludesErrorDetail();
    void lookupSelectionRejectsWhitespaceFallback();
    void lookupSelectionTitlesFollowErrorCode_data();
    void lookupSelectionTitlesFollowErrorCode();
    void lookupSelectionTrimsText();
    void concurrentLookupIsIgnored();
    void showSettingsDelegates();
    void translatesWhenLanguagesDiffer();
    void skipsTranslationForMonolingualLookups();
    void keepsEntryWhenTranslationFails();
};

void ApplicationControllerTest::startRegistersShortcutAndTray() {
    Harness harness;

    harness.controller->start();

    QCOMPARE(harness.shortcutsPtr->registerCalls, 1);
    QCOMPARE(harness.shortcutsPtr->specs.size(), 1);
    const ShortcutSpec spec = harness.shortcutsPtr->specs.value(QStringLiteral("lookup_selection"));
    QCOMPARE(spec.id, QStringLiteral("lookup_selection"));
    QCOMPARE(spec.defaultSequence, harness.config.hotkeys.lookup);
    QVERIFY(!spec.displayName.isEmpty());

    QCOMPARE(harness.trayPtr->startCalls, 1);
    QVERIFY(harness.trayPtr->started);
    QVERIFY(harness.trayPtr->actions.openSettings);
    QVERIFY(harness.trayPtr->actions.lookupSelection);
    QVERIFY(harness.trayPtr->actions.quit);
}

void ApplicationControllerTest::shortcutCallbackLooksUpSelection() {
    Harness harness;
    harness.selectionPtr->result = SelectionResult::success(QStringLiteral("  serendipity  "));
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry());

    harness.controller->start();
    QVERIFY(harness.shortcutsPtr->trigger(QStringLiteral("lookup_selection")));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(0).type, PresenterEvent::Type::Loading);
    QCOMPARE(harness.presenter.events.at(0).title, QStringLiteral("serendipity"));
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Entry);
    QCOMPARE(harness.repositoryPtr->lastWord, QStringLiteral("serendipity"));
}

void ApplicationControllerTest::lookupWordShowsEntry() {
    Harness harness;
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry(QStringLiteral("run"), 3));

    harness.controller->lookupWord(QStringLiteral("  run  "));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(0).type, PresenterEvent::Type::Loading);
    QCOMPARE(harness.presenter.events.at(0).title, QStringLiteral("run"));
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Entry);
    QCOMPARE(harness.presenter.events.at(1).title, QStringLiteral("run"));
    QCOMPARE(harness.presenter.events.at(1).definitionCount, 3);
    QCOMPARE(harness.repositoryPtr->lastWord, QStringLiteral("run"));
}

void ApplicationControllerTest::lookupWordRejectsEmptyInput() {
    Harness harness;

    harness.controller->lookupWord(QStringLiteral(" \t\n "));

    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().type, PresenterEvent::Type::Message);
    QCOMPARE(harness.presenter.events.constFirst().title, QStringLiteral("Nothing to look up"));
    QCOMPARE(harness.repositoryPtr->calls, 0);
}

void ApplicationControllerTest::lookupWordReportsMissingEntry() {
    Harness harness;
    harness.repositoryPtr->result = DictionaryLookupResult::notFound();

    harness.controller->lookupWord(QStringLiteral("missing"));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Message);
    QCOMPARE(harness.presenter.events.at(1).title, QStringLiteral("No entry found"));
    QVERIFY(harness.presenter.events.at(1).message.contains(QStringLiteral("missing")));
}

void ApplicationControllerTest::lookupWordReportsRepositoryFailure() {
    Harness harness;
    harness.repositoryPtr->result = DictionaryLookupResult::failure(
        AppError::make(ErrorCode::DatabaseOpenFailed, QStringLiteral("Could not open."), QStringLiteral("details")));

    harness.controller->lookupWord(QStringLiteral("serendipity"));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Message);
    QCOMPARE(harness.presenter.events.at(1).title, QStringLiteral("Lookup failed"));
    QVERIFY(harness.presenter.events.at(1).message.contains(QStringLiteral("Could not open.")));
    QVERIFY(harness.presenter.events.at(1).message.contains(QStringLiteral("details")));
}

void ApplicationControllerTest::lookupSelectionReportsUnavailable() {
    Harness harness;
    harness.selectionPtr->result = SelectionResult::failure(
        AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("Selection is unavailable.")));

    harness.controller->lookupSelection();

    QCOMPARE(harness.selectionPtr->calls, 1);
    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().type, PresenterEvent::Type::Message);
    QCOMPARE(harness.presenter.events.constFirst().title, QStringLiteral("No selection"));
    QCOMPARE(harness.presenter.events.constFirst().message, QStringLiteral("Selection is unavailable."));
    QCOMPARE(harness.repositoryPtr->calls, 0);
}

void ApplicationControllerTest::lookupSelectionIncludesErrorDetail() {
    Harness harness;
    harness.selectionPtr->result =
        SelectionResult::failure(AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("Unavailable."),
                                                QStringLiteral("Wayland support is pending.")));

    harness.controller->lookupSelection();

    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().message,
             QStringLiteral("Unavailable.\n\nWayland support is pending."));
}

void ApplicationControllerTest::lookupSelectionRejectsWhitespaceFallback() {
    Harness harness;
    harness.selectionPtr->result = SelectionResult::success(QStringLiteral(" \t "));

    harness.controller->lookupSelection();

    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().type, PresenterEvent::Type::Message);
    QCOMPARE(harness.presenter.events.constFirst().title, QStringLiteral("No selection"));
    QVERIFY(!harness.presenter.events.constFirst().message.isEmpty());
    QCOMPARE(harness.repositoryPtr->calls, 0);
}

void ApplicationControllerTest::lookupSelectionTitlesFollowErrorCode_data() {
    QTest::addColumn<ErrorCode>("code");
    QTest::addColumn<QString>("title");

    QTest::newRow("unavailable") << ErrorCode::SelectionUnavailable << QStringLiteral("No selection");
    QTest::newRow("unsupported") << ErrorCode::SelectionUnsupported << QStringLiteral("Selection not supported");
    QTest::newRow("not-text") << ErrorCode::SelectionNotText << QStringLiteral("Selection is not text");
    QTest::newRow("too-long") << ErrorCode::SelectionTooLong << QStringLiteral("Selection too long");
}

void ApplicationControllerTest::lookupSelectionTitlesFollowErrorCode() {
    QFETCH(ErrorCode, code);
    QFETCH(QString, title);

    Harness harness;
    harness.selectionPtr->result = SelectionResult::failure(AppError::make(code, QStringLiteral("Selection failed.")));

    harness.controller->lookupSelection();

    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().title, title);
    QCOMPARE(harness.presenter.events.constFirst().message, QStringLiteral("Selection failed."));
    QCOMPARE(harness.repositoryPtr->calls, 0);
}

void ApplicationControllerTest::lookupSelectionTrimsText() {
    Harness harness;
    harness.selectionPtr->result = SelectionResult::success(QStringLiteral("\n  serendipity \t"));
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry());

    harness.controller->lookupSelection();

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.repositoryPtr->lastWord, QStringLiteral("serendipity"));
}

void ApplicationControllerTest::concurrentLookupIsIgnored() {
    auto repository = std::make_unique<BlockingRepository>();
    BlockingRepository* repositoryPtr = repository.get();
    repositoryPtr->result = DictionaryLookupResult::success(makeEntry());

    auto dictionary = std::make_unique<DictionaryService>(std::move(repository));
    auto selection = std::make_unique<FakeSelectionProvider>();
    auto shortcuts = std::make_unique<FakeShortcutService>();
    auto tray = std::make_unique<FakeTrayService>();
    RecordingPresenter presenter;

    // NOLINTNEXTLINE(readability-trailing-comma): the comma belongs to the argument list.
    ApplicationController controller(Config{}, std::move(dictionary), std::make_unique<FakeTranslationService>(),
                                     std::move(selection), std::move(shortcuts), std::move(tray), presenter);

    controller.lookupWord(QStringLiteral("first"));
    QVERIFY(repositoryPtr->entered.tryAcquire(1, 5000));

    controller.lookupWord(QStringLiteral("second"));
    QCOMPARE(presenter.events.size(), 1);

    repositoryPtr->release.release();
    QTRY_COMPARE_WITH_TIMEOUT(presenter.events.size(), 2, 5000);

    QCOMPARE(repositoryPtr->calls, 1);
    QCOMPARE(repositoryPtr->lastWord, QStringLiteral("first"));
    QCOMPARE(presenter.events.at(1).type, PresenterEvent::Type::Entry);
}

void ApplicationControllerTest::showSettingsDelegates() {
    Harness harness;

    harness.controller->showSettings();

    QCOMPARE(harness.presenter.events.size(), 1);
    QCOMPARE(harness.presenter.events.constFirst().type, PresenterEvent::Type::Settings);
}

void ApplicationControllerTest::translatesWhenLanguagesDiffer() {
    Harness harness(translatedConfig());
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry(QStringLiteral("world")));
    harness.translationPtr->translationResult = QStringLiteral("Welt");

    harness.controller->lookupWord(QStringLiteral("world"));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Entry);
    QCOMPARE(harness.presenter.events.at(1).translation, QStringLiteral("Welt"));
    QCOMPARE(harness.translationPtr->translateCalls, 1);
    QCOMPARE(harness.translationPtr->lastSource, QStringLiteral("en"));
    QCOMPARE(harness.translationPtr->lastTarget, QStringLiteral("de"));
}

void ApplicationControllerTest::skipsTranslationForMonolingualLookups() {
    Harness harness;
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry());

    harness.controller->lookupWord(QStringLiteral("serendipity"));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.translationPtr->translateCalls, 0);
    QVERIFY(harness.presenter.events.at(1).translation.isEmpty());
}

void ApplicationControllerTest::keepsEntryWhenTranslationFails() {
    Harness harness(translatedConfig());
    harness.repositoryPtr->result = DictionaryLookupResult::success(makeEntry());
    harness.translationPtr->translationResult =
        AppError::make(ErrorCode::TranslationUnavailable, QStringLiteral("No German pack installed."));

    harness.controller->lookupWord(QStringLiteral("serendipity"));

    QTRY_COMPARE_WITH_TIMEOUT(harness.presenter.events.size(), 2, 5000);
    QCOMPARE(harness.presenter.events.at(1).type, PresenterEvent::Type::Entry);
    QVERIFY(harness.presenter.events.at(1).translation.isEmpty());
}

QTEST_GUILESS_MAIN(ApplicationControllerTest)

#include "test_application_controller.moc"

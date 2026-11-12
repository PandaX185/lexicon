#pragma once

#include "core/interfaces/PopupPresenter.h"
#include "core/interfaces/SelectionProvider.h"
#include "core/interfaces/ShortcutService.h"
#include "core/interfaces/TranslationService.h"
#include "core/interfaces/TrayService.h"
#include "dictionary/DictionaryRepository.h"
#include "dictionary/DictionaryService.h"
#include "dictionary/models/WordEntry.h"

#include <QHash>
#include <QList>
#include <QSemaphore>
#include <QString>

#include <functional>
#include <memory>
#include <utility>

namespace Lexicon::Test {

struct PresenterEvent {
    enum class Type {
        Loading,
        Entry,
        Message,
        Settings,
        Dismiss,
    };

    Type type = Type::Loading;
    QString title;
    QString message;
    QString translation;
    int definitionCount = 0;
};

class RecordingPresenter final : public PopupPresenter {
  public:
    void showLoading(const QString& word) override {
        events.append(PresenterEvent{
            .type = PresenterEvent::Type::Loading,
            .title = word,
            .message = {},
            .definitionCount = 0,
        });
    }

    void showEntry(const WordEntry& entry) override {
        events.append(PresenterEvent{
            .type = PresenterEvent::Type::Entry,
            .title = entry.word,
            .message = {},
            .translation = entry.metadata.value(QStringLiteral("translation")),
            .definitionCount = static_cast<int>(entry.definitions.size()),
        });
    }

    void showMessage(const QString& title, const QString& message) override {
        events.append(PresenterEvent{
            .type = PresenterEvent::Type::Message,
            .title = title,
            .message = message,
            .definitionCount = 0,
        });
    }

    void showSettings() override {
        events.append(PresenterEvent{
            .type = PresenterEvent::Type::Settings,
            .title = {},
            .message = {},
            .definitionCount = 0,
        });
    }

    void dismiss() override {
        events.append(PresenterEvent{
            .type = PresenterEvent::Type::Dismiss,
            .title = {},
            .message = {},
            .definitionCount = 0,
        });
    }

    [[nodiscard]] const PresenterEvent* last() const {
        return events.isEmpty() ? nullptr : &events.constLast();
    }

    void clear() {
        events.clear();
    }

    QList<PresenterEvent> events;
};

class FakeSelectionProvider final : public SelectionProvider {
  public:
    SelectionResult currentSelection() override {
        ++calls;
        return result;
    }

    [[nodiscard]] QString displayName() const override {
        return QStringLiteral("fake selection provider");
    }

    SelectionResult result =
        SelectionResult::failure(AppError::make(ErrorCode::SelectionUnavailable, QStringLiteral("unavailable")));
    int calls = 0;
};

class FakeShortcutService final : public ShortcutService {
  public:
    ServiceStatus registerShortcut(const ShortcutSpec& spec, std::function<void()> callback) override {
        ++registerCalls;
        if (!registrationResult.available) {
            return registrationResult;
        }
        specs.insert(spec.id, spec);
        callbacks.insert(spec.id, std::move(callback));
        return ServiceStatus::success();
    }

    void unregisterShortcut(const QString& id) override {
        ++unregisterCalls;
        specs.remove(id);
        callbacks.remove(id);
    }

    void unregisterAll() override {
        ++unregisterAllCalls;
        specs.clear();
        callbacks.clear();
    }

    bool trigger(const QString& id) const {
        const auto it = callbacks.constFind(id);
        if (it == callbacks.cend()) {
            return false;
        }
        it.value()();
        return true;
    }

    ServiceStatus registrationResult = ServiceStatus::success();
    QHash<QString, ShortcutSpec> specs;
    QHash<QString, std::function<void()>> callbacks;
    int registerCalls = 0;
    int unregisterCalls = 0;
    int unregisterAllCalls = 0;
};

class FakeTrayService final : public TrayService {
  public:
    ServiceStatus start(const TrayActions& newActions) override {
        ++startCalls;
        if (!startResult.available) {
            return startResult;
        }
        actions = newActions;
        started = true;
        return ServiceStatus::success();
    }

    void stop() override {
        ++stopCalls;
        started = false;
    }

    ServiceStatus startResult = ServiceStatus::success();
    TrayActions actions;
    bool started = false;
    int startCalls = 0;
    int stopCalls = 0;
};

class FakeTranslationService final : public TranslationService {
  public:
    Result<QString> translate(const QString& text, const QString& from, const QString& to) override {
        ++translateCalls;
        lastText = text;
        lastSource = from;
        lastTarget = to;
        return translationResult;
    }

    Result<QList<LanguagePair>> installedPairs() override {
        return pairs;
    }

    bool isPairInstalled(const QString& from, const QString& to) override {
        return pairs.contains(LanguagePair{.source = from, .target = to});
    }

    Result<QString> translationResult = QStringLiteral("translated");
    QList<LanguagePair> pairs;
    QString lastText;
    QString lastSource;
    QString lastTarget;
    int translateCalls = 0;
};

class FakeRepository final : public DictionaryRepository {
  public:
    explicit FakeRepository(DictionaryLookupResult newResult = DictionaryLookupResult::notFound())
        : result(std::move(newResult)) {}

    DictionaryLookupResult findByWord(const QString& word) const override {
        ++calls;
        lastWord = word;
        if (beforeReturn) {
            beforeReturn();
        }
        return result;
    }

    [[nodiscard]] DictionaryDescription describe() const override {
        DictionaryDescription description;
        description.info = DictionaryInfo{
            .name = QStringLiteral("fake"),
            .sourceLanguage = QStringLiteral("en"),
            .targetLanguage = QStringLiteral("en"),
            .license = {},
            .attribution = {},
        };
        description.entryCount = 1;
        return description;
    }

    [[nodiscard]] QString location() const override {
        return QStringLiteral("fake.sqlite");
    }

    mutable DictionaryLookupResult result;
    mutable QString lastWord;
    mutable int calls = 0;
    mutable std::function<void()> beforeReturn;
};

class BlockingRepository final : public DictionaryRepository {
  public:
    DictionaryLookupResult findByWord(const QString& word) const override {
        ++calls;
        lastWord = word;
        entered.release();
        release.acquire();
        return result;
    }

    [[nodiscard]] DictionaryDescription describe() const override {
        DictionaryDescription description;
        description.info = DictionaryInfo{
            .name = QStringLiteral("blocking"),
            .sourceLanguage = QStringLiteral("en"),
            .targetLanguage = QStringLiteral("en"),
            .license = {},
            .attribution = {},
        };
        return description;
    }

    [[nodiscard]] QString location() const override {
        return QStringLiteral("blocking.sqlite");
    }

    mutable DictionaryLookupResult result = DictionaryLookupResult::notFound();
    mutable QString lastWord;
    mutable int calls = 0;
    mutable QSemaphore entered;
    mutable QSemaphore release;
};

inline std::unique_ptr<DictionaryService> makeDictionaryService(std::unique_ptr<DictionaryRepository> repository) {
    return std::make_unique<DictionaryService>(std::move(repository));
}

} // namespace Lexicon::Test

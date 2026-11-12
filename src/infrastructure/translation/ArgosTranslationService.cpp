#include "infrastructure/translation/ArgosTranslationService.h"

#include "infrastructure/translation/ArgosProcessWorker.h"

#include <QJsonArray>
#include <QJsonObject>

#include <algorithm>
#include <utility>

namespace Lexicon {

ArgosTranslationService::ArgosTranslationService(std::unique_ptr<ArgosProcessWorker> worker)
    : m_worker(worker ? std::move(worker) : std::make_unique<ArgosProcessWorker>()) {}

ArgosTranslationService::~ArgosTranslationService() = default;

Result<QString> ArgosTranslationService::translate(const QString& text, const QString& from, const QString& to) {
    const QString trimmed = text.trimmed();
    if (trimmed.isEmpty()) {
        return AppError::make(ErrorCode::InvalidArgument, QStringLiteral("There is nothing to translate."));
    }
    if (from.isEmpty() || to.isEmpty()) {
        return AppError::make(ErrorCode::InvalidArgument, QStringLiteral("A source and target language are required."));
    }
    if (from == to) {
        return trimmed;
    }

    const QJsonObject params{
        {QStringLiteral("text"), trimmed},
        {QStringLiteral("from"), from},
        {QStringLiteral("to"), to},
    };
    const Result<QJsonObject> response = m_worker->sendRequest(QStringLiteral("translate"), params);
    if (!response.hasValue()) {
        return response.error();
    }

    const QString translated = response->value(QStringLiteral("translatedText")).toString();
    if (translated.isEmpty()) {
        return AppError::make(ErrorCode::TranslationUnavailable,
                              QStringLiteral("The translation backend returned no text."));
    }
    return translated;
}

Result<QList<LanguagePair>> ArgosTranslationService::installedPairs() {
    const Result<QJsonObject> response = m_worker->sendRequest(QStringLiteral("list_installed_pairs"), {});
    if (!response.hasValue()) {
        return response.error();
    }

    QList<LanguagePair> pairs;
    const QJsonArray array = response->value(QStringLiteral("pairs")).toArray();
    pairs.reserve(array.size());
    for (const auto& value : array) {
        const QJsonObject item = value.toObject();
        LanguagePair pair{
            .source = item.value(QStringLiteral("from")).toString(),
            .target = item.value(QStringLiteral("to")).toString(),
            .sourceName = item.value(QStringLiteral("from_name")).toString(),
            .targetName = item.value(QStringLiteral("to_name")).toString(),
        };
        if (pair.isValid()) {
            pairs.append(std::move(pair));
        }
    }
    return pairs;
}

Result<bool> ArgosTranslationService::installFromArchive(const QString& archivePath) {
    if (archivePath.isEmpty()) {
        return AppError::make(ErrorCode::InvalidArgument, QStringLiteral("No language pack archive was given."));
    }

    const QJsonObject params{{QStringLiteral("path"), archivePath}};
    const Result<QJsonObject> response = m_worker->sendRequest(QStringLiteral("install_model"), params, 60000);
    if (!response.hasValue()) {
        return response.error();
    }
    return response->value(QStringLiteral("installed")).toBool();
}

Result<bool> ArgosTranslationService::uninstallPair(const QString& from, const QString& to) {
    if (from.isEmpty() || to.isEmpty()) {
        return AppError::make(ErrorCode::InvalidArgument, QStringLiteral("A language pair is required."));
    }

    const QJsonObject params{
        {QStringLiteral("from"), from},
        {QStringLiteral("to"), to},
    };
    const Result<QJsonObject> response = m_worker->sendRequest(QStringLiteral("uninstall_model"), params, 30000);
    if (!response.hasValue()) {
        return response.error();
    }
    return response->value(QStringLiteral("uninstalled")).toBool();
}

bool ArgosTranslationService::isPairInstalled(const QString& from, const QString& to) {
    const Result<QList<LanguagePair>> pairs = installedPairs();
    if (!pairs.hasValue()) {
        return false;
    }
    return std::any_of(pairs->cbegin(), pairs->cend(),
                       [&from, &to](const LanguagePair& pair) { return pair.source == from && pair.target == to; });
}

} // namespace Lexicon

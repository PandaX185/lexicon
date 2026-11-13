#include "config/ConfigLoader.h"

#include "infrastructure/logging/Logging.h"

#include <QDir>
#include <QFileInfo>
#include <QSaveFile>
#include <QSet>
#include <QStandardPaths>
#include <QStringList>
#include <toml++/toml.hpp>

#include <algorithm>
#include <cstdint>
#include <limits>
#include <string>
#include <string_view>
#include <utility>

namespace {

using Lexicon::Animation;
using Lexicon::ConfigIssue;
using Lexicon::ConfigIssueSeverity;
using Lexicon::ConfigLoadResult;
using Lexicon::PopupPosition;
using Lexicon::PronunciationStyle;
using Lexicon::Theme;

QString fromStringView(std::string_view value) {
    return QString::fromUtf8(value.data(), static_cast<int>(value.size()));
}

void addIssue(ConfigLoadResult& result, ConfigIssueSeverity severity, const QString& key, const QString& message) {
    result.issues.append(ConfigIssue{.severity = severity, .key = key, .message = message});
}

void addWarning(ConfigLoadResult& result, const QString& key, const QString& message) {
    addIssue(result, ConfigIssueSeverity::Warning, key, message);
}

void addError(ConfigLoadResult& result, const QString& key, const QString& message) {
    addIssue(result, ConfigIssueSeverity::Error, key, message);
}

const toml::table* readSection(const toml::table& root, const char* name, const QString& key,
                               ConfigLoadResult& result) {
    const toml::node* node = root.get(name);
    if (!node) {
        return nullptr;
    }
    const toml::table* table = node->as_table();
    if (!table) {
        addWarning(result, key, QStringLiteral("Expected a TOML table; keeping defaults."));
    }
    return table;
}

bool readBool(const toml::table* table, const char* key, bool& target, ConfigLoadResult& result,
              const QString& prefix) {
    if (!table) {
        return false;
    }
    const toml::node* node = table->get(key);
    if (!node) {
        return false;
    }
    // toml++'s value<bool>() also converts integers, so check the node type explicitly.
    if (node->is_boolean()) {
        if (const auto value = node->value<bool>()) {
            target = *value;
            return true;
        }
    }
    addWarning(result, prefix + QLatin1Char('.') + QString::fromLatin1(key),
               QStringLiteral("Expected a boolean; keeping default."));
    return false;
}

bool readString(const toml::table* table, const char* key, QString& target, ConfigLoadResult& result,
                const QString& prefix);

// Language tags are stored as lowercase BCP 47, so accept "EN", "en_GB" and
// "en-GB" in the file and normalize them instead of rejecting the value.
void readLanguageTag(const toml::table& table, const char* key, QString& target, ConfigLoadResult& result) {
    QString value;
    if (!readString(&table, key, value, result, QStringLiteral("dictionary"))) {
        return;
    }
    const QString normalized = value.trimmed().toLower().replace(QLatin1Char('_'), QLatin1Char('-'));
    if (normalized.isEmpty()) {
        addWarning(result, QStringLiteral("dictionary.") + QString::fromLatin1(key),
                   QStringLiteral("Expected a language tag; keeping the default."));
        return;
    }
    target = normalized;
}

bool readString(const toml::table* table, const char* key, QString& target, ConfigLoadResult& result,
                const QString& prefix) {
    if (!table) {
        return false;
    }
    const toml::node* node = table->get(key);
    if (!node) {
        return false;
    }
    if (node->is_string()) {
        if (const auto value = node->value<std::string>()) {
            target = QString::fromStdString(*value);
            return true;
        }
    }
    addWarning(result, prefix + QLatin1Char('.') + QString::fromLatin1(key),
               QStringLiteral("Expected a string; keeping default."));
    return false;
}

bool readInt(const toml::table* table, const char* key, int& target, ConfigLoadResult& result, const QString& prefix) {
    if (!table) {
        return false;
    }
    const toml::node* node = table->get(key);
    if (!node) {
        return false;
    }
    // toml++'s value<int64_t>() also converts booleans, so check the node type explicitly.
    if (node->is_integer()) {
        if (const auto value = node->value<std::int64_t>()) {
            if (*value < std::numeric_limits<int>::min() || *value > std::numeric_limits<int>::max()) {
                addWarning(result, prefix + QLatin1Char('.') + QString::fromLatin1(key),
                           QStringLiteral("Integer is out of range; keeping default."));
                return false;
            }
            target = static_cast<int>(*value);
            return true;
        }
    }
    addWarning(result, prefix + QLatin1Char('.') + QString::fromLatin1(key),
               QStringLiteral("Expected an integer; keeping default."));
    return false;
}

QSet<QString> knownKeysForSection(const QString& section) {
    if (section == QStringLiteral("general")) {
        return {QStringLiteral("start_on_login"), QStringLiteral("offline_first")};
    }
    if (section == QStringLiteral("interaction")) {
        return {
            QStringLiteral("activation"),
            QStringLiteral("clipboard_fallback"),
            QStringLiteral("dismiss_on_escape"),
            QStringLiteral("dismiss_on_outside_click"),
        };
    }
    if (section == QStringLiteral("hotkeys")) {
        return {QStringLiteral("lookup")};
    }
    if (section == QStringLiteral("appearance")) {
        return {
            QStringLiteral("theme"),
            QStringLiteral("popup_width"),
            QStringLiteral("animation"),
            QStringLiteral("position"),
        };
    }
    if (section == QStringLiteral("dictionary")) {
        return {
            QStringLiteral("database_path"),      QStringLiteral("directory"),
            QStringLiteral("source_language"),    QStringLiteral("target_language"),
            QStringLiteral("show_pronunciation"), QStringLiteral("pronunciation_style"),
        };
    }
    if (section == QStringLiteral("translation")) {
        return {QStringLiteral("catalog_url")};
    }
    if (section == QStringLiteral("performance")) {
        return {QStringLiteral("cache_enabled"), QStringLiteral("cache_size"), QStringLiteral("lazy_initialize")};
    }
    return {};
}

bool isKnownSection(const QString& section) {
    static const QSet<QString> sections{
        QStringLiteral("general"),     QStringLiteral("interaction"), QStringLiteral("hotkeys"),
        QStringLiteral("appearance"),  QStringLiteral("dictionary"),  QStringLiteral("translation"),
        QStringLiteral("performance"),
    };
    return sections.contains(section);
}

// Saving rewrites the whole document, so anything the schema does not know
// about would be lost. Detect that first and keep a backup instead of silently
// discarding the user's comments or hand-written keys.
void reportUnsavedContent(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly | QIODevice::Text)) {
        return;
    }
    const QString text = QString::fromUtf8(file.readAll());
    file.close();

    QStringList discarded;
    const QStringList lines = text.split(QLatin1Char('\n'));
    for (const QString& line : lines) {
        if (line.trimmed().startsWith(QLatin1Char('#'))) {
            discarded.append(QStringLiteral("comments"));
            break;
        }
    }

    const toml::parse_result parsed = toml::parse_file(path.toStdString());
    if (parsed) {
        for (const auto& entry : parsed.table()) {
            const QString key = fromStringView(entry.first.str());
            if (!isKnownSection(key)) {
                discarded.append(key);
                continue;
            }
            const toml::table* section = entry.second.as_table();
            if (!section) {
                discarded.append(key);
                continue;
            }
            const QSet<QString> known = knownKeysForSection(key);
            for (const auto& sectionEntry : *section) {
                const QString sectionKey = fromStringView(sectionEntry.first.str());
                if (!known.contains(sectionKey)) {
                    discarded.append(key + QLatin1Char('.') + sectionKey);
                }
            }
        }
    }

    if (discarded.isEmpty()) {
        return;
    }

    const QString backup = path + QStringLiteral(".bak");
    QFile::remove(backup);
    if (!QFile::copy(path, backup)) {
        qCWarning(lexiconConfig) << "event=config_backup_failed path=" << path << "backup=" << backup;
        return;
    }
    qCWarning(lexiconConfig) << "event=config_save_discards_content path=" << path << "backup=" << backup
                             << "items=" << discarded.join(QStringLiteral(", "));
}

void warnUnknownKeys(const toml::table& table, const QString& prefix, const QSet<QString>& known,
                     ConfigLoadResult& result) {
    for (const auto& entry : table) {
        const QString key = fromStringView(entry.first.str());
        if (!known.contains(key)) {
            addWarning(result, prefix.isEmpty() ? key : prefix + QLatin1Char('.') + key,
                       QStringLiteral("Unknown key; ignored."));
        }
    }
}

void readInteraction(const toml::table* table, ConfigLoadResult& result) {
    if (!table) {
        return;
    }
    warnUnknownKeys(*table, QStringLiteral("interaction"),
                    {
                        QStringLiteral("activation"),
                        QStringLiteral("clipboard_fallback"),
                        QStringLiteral("dismiss_on_escape"),
                        QStringLiteral("dismiss_on_outside_click"),
                    },
                    result);

    QString activation;
    if (readString(table, "activation", activation, result, QStringLiteral("interaction"))) {
        if (activation == QStringLiteral("global_shortcut")) {
            result.config.interaction.activation = Lexicon::Activation::GlobalShortcut;
        } else {
            addWarning(result, QStringLiteral("interaction.activation"),
                       QStringLiteral("Unsupported activation; keeping global_shortcut."));
        }
    }
    readBool(table, "clipboard_fallback", result.config.interaction.clipboardFallback, result,
             QStringLiteral("interaction"));
    readBool(table, "dismiss_on_escape", result.config.interaction.dismissOnEscape, result,
             QStringLiteral("interaction"));
    readBool(table, "dismiss_on_outside_click", result.config.interaction.dismissOnOutsideClick, result,
             QStringLiteral("interaction"));
}

void readAppearance(const toml::table* table, ConfigLoadResult& result) {
    if (!table) {
        return;
    }
    warnUnknownKeys(*table, QStringLiteral("appearance"),
                    {
                        QStringLiteral("theme"),
                        QStringLiteral("popup_width"),
                        QStringLiteral("animation"),
                        QStringLiteral("position"),
                    },
                    result);

    QString theme;
    if (readString(table, "theme", theme, result, QStringLiteral("appearance"))) {
        if (theme == QStringLiteral("dark")) {
            result.config.appearance.theme = Theme::Dark;
        } else if (theme == QStringLiteral("light")) {
            result.config.appearance.theme = Theme::Light;
        } else if (theme == QStringLiteral("system")) {
            result.config.appearance.theme = Theme::System;
        } else {
            addWarning(result, QStringLiteral("appearance.theme"), QStringLiteral("Unsupported theme; keeping dark."));
        }
    }

    readInt(table, "popup_width", result.config.appearance.popupWidth, result, QStringLiteral("appearance"));

    QString animation;
    if (readString(table, "animation", animation, result, QStringLiteral("appearance"))) {
        if (animation == QStringLiteral("none")) {
            result.config.appearance.animation = Animation::None;
        } else if (animation == QStringLiteral("subtle")) {
            result.config.appearance.animation = Animation::Subtle;
        } else {
            addWarning(result, QStringLiteral("appearance.animation"),
                       QStringLiteral("Unsupported animation; keeping subtle."));
        }
    }

    QString position;
    if (readString(table, "position", position, result, QStringLiteral("appearance"))) {
        if (position == QStringLiteral("near_selection")) {
            result.config.appearance.position = PopupPosition::NearSelection;
        } else if (position == QStringLiteral("center")) {
            result.config.appearance.position = PopupPosition::Center;
        } else if (position == QStringLiteral("cursor")) {
            result.config.appearance.position = PopupPosition::Cursor;
        } else {
            addWarning(result, QStringLiteral("appearance.position"),
                       QStringLiteral("Unsupported position; keeping near_selection."));
        }
    }
}

} // namespace

namespace Lexicon {

ConfigLoadResult ConfigLoader::load(const QString& explicitPath) const {
    ConfigLoadResult result;

    QString path = explicitPath;
    if (path.isEmpty()) {
        path = qEnvironmentVariable("LEXICON_CONFIG");
    }
    if (path.isEmpty()) {
        path = defaultConfigPath();
    }
    result.sourcePath = path;
    result.config.configPath = path;

    const QFileInfo configFile(path);
    if (!configFile.exists()) {
        qCInfo(lexiconConfig) << "event=config_missing path=" << path << "action=use_defaults";
        return result;
    }

    result.found = true;

    // toml::parse_file() opens with std::ifstream; whether that fails for a
    // directory is platform- and filesystem-dependent. Some environments let
    // the open succeed and then parse zero bytes as a valid empty document, so
    // reject non-regular files explicitly before parsing.
    if (!configFile.isFile()) {
        addError(result, QStringLiteral("<file>"), QStringLiteral("Configuration path is not a regular file."));
        result.config = Config{};
        result.config.configPath = path;
        result.usedDefaults = true;
        qCWarning(lexiconConfig) << "event=config_not_a_file path=" << path;
        return result;
    }

    const toml::parse_result parsed = toml::parse_file(path.toStdString());
    if (!parsed) {
        const toml::parse_error& error = parsed.error();
        const std::string_view description = error.description();
        const QString message = QString::fromUtf8(description.data(), static_cast<int>(description.size()));
        addError(result, QStringLiteral("<file>"), QStringLiteral("Could not parse configuration: %1").arg(message));
        result.config = Config{};
        result.config.configPath = path;
        result.usedDefaults = true;
        qCWarning(lexiconConfig) << "event=config_parse_failed path=" << path << "detail=" << message;
    } else {
        const toml::table& root = parsed.table();

        warnUnknownKeys(root, {},
                        {
                            QStringLiteral("general"),
                            QStringLiteral("interaction"),
                            QStringLiteral("hotkeys"),
                            QStringLiteral("appearance"),
                            QStringLiteral("dictionary"),
                            QStringLiteral("translation"),
                            QStringLiteral("performance"),
                        },
                        result);

        const toml::table* general = readSection(root, "general", QStringLiteral("general"), result);
        if (general) {
            warnUnknownKeys(*general, QStringLiteral("general"),
                            {QStringLiteral("start_on_login"), QStringLiteral("offline_first")}, result);
            readBool(general, "start_on_login", result.config.general.startOnLogin, result, QStringLiteral("general"));
            readBool(general, "offline_first", result.config.general.offlineFirst, result, QStringLiteral("general"));
        }

        readInteraction(readSection(root, "interaction", QStringLiteral("interaction"), result), result);

        const toml::table* hotkeys = readSection(root, "hotkeys", QStringLiteral("hotkeys"), result);
        if (hotkeys) {
            warnUnknownKeys(*hotkeys, QStringLiteral("hotkeys"), {QStringLiteral("lookup")}, result);
            QString lookup;
            if (readString(hotkeys, "lookup", lookup, result, QStringLiteral("hotkeys")) && !lookup.isEmpty()) {
                result.config.hotkeys.lookup = lookup;
            }
        }

        readAppearance(readSection(root, "appearance", QStringLiteral("appearance"), result), result);

        const toml::table* dictionary = readSection(root, "dictionary", QStringLiteral("dictionary"), result);
        if (dictionary) {
            warnUnknownKeys(*dictionary, QStringLiteral("dictionary"),
                            {
                                QStringLiteral("database_path"),
                                QStringLiteral("directory"),
                                QStringLiteral("source_language"),
                                QStringLiteral("target_language"),
                                QStringLiteral("show_pronunciation"),
                                QStringLiteral("pronunciation_style"),
                            },
                            result);
            readString(dictionary, "database_path", result.config.dictionary.databasePath, result,
                       QStringLiteral("dictionary"));
            readString(dictionary, "directory", result.config.dictionary.directory, result,
                       QStringLiteral("dictionary"));
            readLanguageTag(*dictionary, "source_language", result.config.dictionary.sourceLanguage, result);
            readLanguageTag(*dictionary, "target_language", result.config.dictionary.targetLanguage, result);
            readBool(dictionary, "show_pronunciation", result.config.dictionary.showPronunciation, result,
                     QStringLiteral("dictionary"));

            QString style;
            if (readString(dictionary, "pronunciation_style", style, result, QStringLiteral("dictionary"))) {
                if (style == QStringLiteral("respelling")) {
                    result.config.dictionary.pronunciationStyle = PronunciationStyle::Respelling;
                } else if (style == QStringLiteral("ipa")) {
                    result.config.dictionary.pronunciationStyle = PronunciationStyle::Ipa;
                } else {
                    addWarning(result, QStringLiteral("dictionary.pronunciation_style"),
                               QStringLiteral("Unsupported pronunciation style; keeping respelling."));
                }
            }
        }

        const toml::table* translation = readSection(root, "translation", QStringLiteral("translation"), result);
        if (translation) {
            warnUnknownKeys(*translation, QStringLiteral("translation"), {QStringLiteral("catalog_url")}, result);
            QString catalogUrl;
            if (readString(translation, "catalog_url", catalogUrl, result, QStringLiteral("translation"))) {
                result.config.translation.catalogUrl = catalogUrl.trimmed();
            }
        }

        const toml::table* performance = readSection(root, "performance", QStringLiteral("performance"), result);
        if (performance) {
            warnUnknownKeys(
                *performance, QStringLiteral("performance"),
                {QStringLiteral("cache_enabled"), QStringLiteral("cache_size"), QStringLiteral("lazy_initialize")},
                result);
            readBool(performance, "cache_enabled", result.config.performance.cacheEnabled, result,
                     QStringLiteral("performance"));
            readInt(performance, "cache_size", result.config.performance.cacheSize, result,
                    QStringLiteral("performance"));
            readBool(performance, "lazy_initialize", result.config.performance.lazyInitialize, result,
                     QStringLiteral("performance"));
        }

        result.config.performance.cacheSize = std::clamp(result.config.performance.cacheSize, 0, 4096);

        if (result.config.appearance.popupWidth < 240 || result.config.appearance.popupWidth > 1200) {
            addWarning(result, QStringLiteral("appearance.popup_width"),
                       QStringLiteral("popup_width must be between 240 and 1200; clamping."));
            result.config.appearance.popupWidth = qBound(240, result.config.appearance.popupWidth, 1200);
        }

        if (result.config.lookupKeySequence().isEmpty()) {
            addWarning(result, QStringLiteral("hotkeys.lookup"),
                       QStringLiteral("Invalid key sequence; keeping SUPER+SHIFT+D."));
            result.config.hotkeys.lookup = QStringLiteral("SUPER+SHIFT+D");
        }

        if (!result.config.dictionary.databasePath.trimmed().isEmpty() &&
            QDir::isRelativePath(result.config.dictionary.databasePath)) {
            result.config.dictionary.databasePath =
                configFile.absoluteDir().absoluteFilePath(result.config.dictionary.databasePath);
        }

        if (!result.config.dictionary.directory.trimmed().isEmpty() &&
            QDir::isRelativePath(result.config.dictionary.directory)) {
            result.config.dictionary.directory =
                configFile.absoluteDir().absoluteFilePath(result.config.dictionary.directory);
        }

        result.usedDefaults = false;
    }

    for (const ConfigIssue& issue : std::as_const(result.issues)) {
        if (issue.isError()) {
            qCWarning(lexiconConfig) << "event=config_issue severity=error key=" << issue.key
                                     << "message=" << issue.message;
        } else {
            qCWarning(lexiconConfig) << "event=config_issue severity=warning key=" << issue.key
                                     << "message=" << issue.message;
        }
    }

    return result;
}

bool ConfigLoader::save(const Config& config, const QString& path) const {
    QString target = path;
    if (target.isEmpty()) {
        target = config.configPath.isEmpty() ? defaultConfigPath() : config.configPath;
    }
    QDir().mkpath(QFileInfo(target).absolutePath());

    const auto quote = [](const QString& value) {
        QString escaped = value;
        escaped.replace(QStringLiteral("\\"), QStringLiteral("\\\\"));
        escaped.replace(QStringLiteral("\""), QStringLiteral("\\\""));
        escaped.replace(QStringLiteral("\n"), QStringLiteral("\\n"));
        return QStringLiteral("\"%1\"").arg(escaped);
    };
    const auto boolStr = [](bool value) { return value ? QStringLiteral("true") : QStringLiteral("false"); };

    QString out;
    out += QStringLiteral("# Lexicon configuration\n\n");

    out += QStringLiteral("[general]\n");
    out += QStringLiteral("start_on_login = %1\n").arg(boolStr(config.general.startOnLogin));
    out += QStringLiteral("offline_first = %1\n\n").arg(boolStr(config.general.offlineFirst));

    out += QStringLiteral("[interaction]\n");
    out += QStringLiteral("activation = %1\n").arg(quote(activationToString(config.interaction.activation)));
    out += QStringLiteral("clipboard_fallback = %1\n").arg(boolStr(config.interaction.clipboardFallback));
    out += QStringLiteral("dismiss_on_escape = %1\n").arg(boolStr(config.interaction.dismissOnEscape));
    out += QStringLiteral("dismiss_on_outside_click = %1\n\n").arg(boolStr(config.interaction.dismissOnOutsideClick));

    out += QStringLiteral("[hotkeys]\n");
    out += QStringLiteral("lookup = %1\n\n").arg(quote(config.hotkeys.lookup));

    out += QStringLiteral("[appearance]\n");
    out += QStringLiteral("theme = %1\n").arg(quote(themeToString(config.appearance.theme)));
    out += QStringLiteral("popup_width = %1\n").arg(config.appearance.popupWidth);
    out += QStringLiteral("animation = %1\n").arg(quote(animationToString(config.appearance.animation)));
    out += QStringLiteral("position = %1\n\n").arg(quote(popupPositionToString(config.appearance.position)));

    out += QStringLiteral("[dictionary]\n");
    out += QStringLiteral("database_path = %1\n").arg(quote(config.dictionary.databasePath));
    out += QStringLiteral("directory = %1\n").arg(quote(config.dictionary.directory));
    out += QStringLiteral("source_language = %1\n").arg(quote(config.dictionary.sourceLanguage));
    out += QStringLiteral("target_language = %1\n").arg(quote(config.dictionary.targetLanguage));
    out += QStringLiteral("show_pronunciation = %1\n").arg(boolStr(config.dictionary.showPronunciation));
    out += QStringLiteral("pronunciation_style = %1\n\n")
               .arg(quote(pronunciationStyleToString(config.dictionary.pronunciationStyle)));

    out += QStringLiteral("[translation]\n");
    out += QStringLiteral("catalog_url = %1\n\n").arg(quote(config.translation.catalogUrl));

    out += QStringLiteral("[performance]\n");
    out += QStringLiteral("cache_enabled = %1\n").arg(boolStr(config.performance.cacheEnabled));
    out += QStringLiteral("cache_size = %1\n").arg(config.performance.cacheSize);
    out += QStringLiteral("lazy_initialize = %1\n").arg(boolStr(config.performance.lazyInitialize));

    if (QFileInfo::exists(target)) {
        reportUnsavedContent(target);
    }

    // QSaveFile writes to a temporary file and renames on commit, so a failed
    // or interrupted save never leaves a truncated configuration behind.
    QSaveFile file(target);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Text)) {
        qCWarning(lexiconConfig) << "event=config_save_failed path=" << target << "error=" << file.errorString();
        return false;
    }
    if (file.write(out.toUtf8()) < 0) {
        qCWarning(lexiconConfig) << "event=config_save_failed path=" << target << "error=" << file.errorString();
        file.cancelWriting();
        return false;
    }
    if (!file.commit()) {
        qCWarning(lexiconConfig) << "event=config_save_failed path=" << target << "error=" << file.errorString();
        return false;
    }
    qCInfo(lexiconConfig) << "event=config_saved path=" << target;
    return true;
}

QString ConfigLoader::defaultConfigPath() {
    return QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation) +
           QStringLiteral("/lexicon/config.toml");
}

QString ConfigLoader::defaultDataDir() {
    return QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation) + QStringLiteral("/lexicon");
}

QString ConfigLoader::defaultDatabasePath() {
    return defaultDataDir() + QStringLiteral("/dictionaries/en.sqlite");
}

} // namespace Lexicon

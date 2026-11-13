#pragma once

#include "config/Config.h"

#include <QList>
#include <QString>

namespace Lexicon {

enum class ConfigIssueSeverity {
    Warning,
    Error,
};

struct ConfigIssue {
    ConfigIssueSeverity severity = ConfigIssueSeverity::Warning;
    QString key;
    QString message;

    [[nodiscard]] bool isError() const noexcept {
        return severity == ConfigIssueSeverity::Error;
    }
};

struct ConfigLoadResult {
    Config config;
    QList<ConfigIssue> issues;
    bool found = false;
    bool usedDefaults = true;
    QString sourcePath;
};

class ConfigLoader {
  public:
    [[nodiscard]] ConfigLoadResult load(const QString& explicitPath = {}) const;
    [[nodiscard]] bool save(const Config& config, const QString& path = {}) const;

    [[nodiscard]] static QString defaultConfigPath();
    [[nodiscard]] static QString defaultDataDir();
    [[nodiscard]] static QString defaultDatabasePath();
};

} // namespace Lexicon

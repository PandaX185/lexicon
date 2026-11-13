#pragma once

#include <QString>

namespace Lexicon {

// Owns the per-user XDG autostart entry. Keeping the file handling here (rather
// than inside ApplicationController) makes the install/remove behaviour testable
// with temporary paths.
class AutostartManager {
  public:
    AutostartManager(QString targetPath, QString templatePath);

    // Resolves the standard autostart target and the installed template.
    [[nodiscard]] static AutostartManager forCurrentUser();

    // Writes the template to `targetPath`, replacing Exec with `executablePath`.
    [[nodiscard]] bool install(const QString& executablePath) const;

    // Removes the entry. Missing entries are treated as success.
    [[nodiscard]] bool remove() const;

    [[nodiscard]] QString targetPath() const {
        return m_targetPath;
    }

  private:
    QString m_targetPath;
    QString m_templatePath;
};

} // namespace Lexicon

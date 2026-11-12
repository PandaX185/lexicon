#pragma once

#include <QByteArray>
#include <QString>

namespace Lexicon::Test {

class ScopedEnvironment {
  public:
    ScopedEnvironment(const char* name, const QByteArray& value)
        : m_name(name), m_hadValue(qEnvironmentVariableIsSet(name)), m_oldValue(qgetenv(name)) {
        qputenv(m_name, value);
    }

    ~ScopedEnvironment() {
        if (m_hadValue) {
            qputenv(m_name, m_oldValue);
        } else {
            qunsetenv(m_name);
        }
    }

    ScopedEnvironment(const ScopedEnvironment&) = delete;
    ScopedEnvironment& operator=(const ScopedEnvironment&) = delete;

  private:
    const char* m_name;
    bool m_hadValue = false;
    QByteArray m_oldValue;
};

} // namespace Lexicon::Test

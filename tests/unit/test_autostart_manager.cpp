#include "app/AutostartManager.h"

#include <QDir>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>

using namespace Lexicon;

namespace {

QString writeFile(const QString& path, const QString& contents) {
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        return {};
    }
    file.write(contents.toUtf8());
    file.close();
    return path;
}

QString readFile(const QString& path) {
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly)) {
        return {};
    }
    return QString::fromUtf8(file.readAll());
}

const QString TemplateContents = QStringLiteral(R"([Desktop Entry]
Type=Application
Name=Lexicon
Exec=lexicon
Icon=lexicon
)");

} // namespace

class AutostartManagerTest : public QObject {
    Q_OBJECT

  private Q_SLOTS:
    void installsEntryAndRewritesExec();
    void createsParentDirectory();
    void quotesExecutablePathWithSpaces();
    void overwritesExistingEntry();
    void failsWhenTemplateIsMissing();
    void removesEntry();
    void removingMissingEntrySucceeds();
    void exposesTargetPath();
};

void AutostartManagerTest::installsEntryAndRewritesExec() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString templatePath = writeFile(directory.filePath(QStringLiteral("template.desktop")), TemplateContents);
    const QString targetPath = directory.filePath(QStringLiteral("autostart/lexicon.desktop"));
    QVERIFY(!templatePath.isEmpty());

    const AutostartManager manager(targetPath, templatePath);
    QVERIFY(manager.install(QStringLiteral("/opt/lexicon/bin/lexicon")));

    const QString content = readFile(targetPath);
    QVERIFY(content.contains(QStringLiteral("Exec=\"/opt/lexicon/bin/lexicon\"")));
    QVERIFY(content.contains(QStringLiteral("Name=Lexicon")));
    QCOMPARE(readFile(templatePath), TemplateContents);
}

void AutostartManagerTest::createsParentDirectory() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString templatePath = writeFile(directory.filePath(QStringLiteral("template.desktop")), TemplateContents);
    const QString targetPath = directory.filePath(QStringLiteral("a/b/c/lexicon.desktop"));

    const AutostartManager manager(targetPath, templatePath);
    QVERIFY(manager.install(QStringLiteral("/usr/bin/lexicon")));
    QVERIFY(QFile::exists(targetPath));
}

void AutostartManagerTest::quotesExecutablePathWithSpaces() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString templatePath = writeFile(directory.filePath(QStringLiteral("template.desktop")), TemplateContents);
    const QString targetPath = directory.filePath(QStringLiteral("lexicon.desktop"));

    const AutostartManager manager(targetPath, templatePath);
    QVERIFY(manager.install(QStringLiteral("/opt/Lexicon App/lexicon")));
    QVERIFY(readFile(targetPath).contains(QStringLiteral("Exec=\"/opt/Lexicon App/lexicon\"")));
}

void AutostartManagerTest::overwritesExistingEntry() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString templatePath = writeFile(directory.filePath(QStringLiteral("template.desktop")), TemplateContents);
    const QString targetPath =
        writeFile(directory.filePath(QStringLiteral("lexicon.desktop")), QStringLiteral("stale entry"));

    const AutostartManager manager(targetPath, templatePath);
    QVERIFY(manager.install(QStringLiteral("/usr/bin/lexicon")));
    const QString content = readFile(targetPath);
    QVERIFY(!content.contains(QStringLiteral("stale entry")));
    QVERIFY(content.contains(QStringLiteral("Exec=\"/usr/bin/lexicon\"")));
}

void AutostartManagerTest::failsWhenTemplateIsMissing() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString targetPath = directory.filePath(QStringLiteral("lexicon.desktop"));

    const AutostartManager manager(targetPath, {});
    QVERIFY(!manager.install(QStringLiteral("/usr/bin/lexicon")));
    QVERIFY(!QFile::exists(targetPath));
}

void AutostartManagerTest::removesEntry() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString targetPath =
        writeFile(directory.filePath(QStringLiteral("lexicon.desktop")), QStringLiteral("entry"));

    const AutostartManager manager(targetPath, {});
    QVERIFY(manager.remove());
    QVERIFY(!QFile::exists(targetPath));
}

void AutostartManagerTest::removingMissingEntrySucceeds() {
    QTemporaryDir directory;
    QVERIFY(directory.isValid());

    const AutostartManager manager(directory.filePath(QStringLiteral("absent.desktop")), {});
    QVERIFY(manager.remove());
}

void AutostartManagerTest::exposesTargetPath() {
    AutostartManager manager(QStringLiteral("/tmp/example.desktop"), {});
    QCOMPARE(manager.targetPath(), QStringLiteral("/tmp/example.desktop"));
}

QTEST_GUILESS_MAIN(AutostartManagerTest)

#include "test_autostart_manager.moc"

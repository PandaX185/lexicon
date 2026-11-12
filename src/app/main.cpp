#include "app/Application.h"
#include "config/ConfigLoader.h"
#include "dictionary/DictionaryCatalog.h"
#include "infrastructure/database/SqliteDictionaryRepository.h"
#include "infrastructure/logging/Logging.h"

#include <QApplication>
#include <QCommandLineOption>
#include <QCommandLineParser>
#include <QCoreApplication>
#include <QTextStream>

#include <memory>

int main(int argc, char* argv[]) {
    QApplication app(argc, argv);
    QCoreApplication::setApplicationName(QStringLiteral("lexicon"));
    QCoreApplication::setApplicationVersion(QStringLiteral("0.1.0"));
    QCoreApplication::setOrganizationName(QStringLiteral("Lexicon"));
    QGuiApplication::setDesktopFileName(QStringLiteral("org.lexicon.Lexicon"));
    QGuiApplication::setQuitOnLastWindowClosed(false);

    QCommandLineParser parser;
    parser.setApplicationDescription(QStringLiteral("Lightweight offline English dictionary for KDE Plasma."));
    parser.addHelpOption();
    parser.addVersionOption();

    QCommandLineOption configOption({QStringLiteral("c"), QStringLiteral("config")},
                                    QStringLiteral("Read configuration from <path>."), QStringLiteral("path"));
    QCommandLineOption databaseOption(QStringLiteral("database"),
                                      QStringLiteral("Override the dictionary database path."), QStringLiteral("path"));
    QCommandLineOption lookupOption({QStringLiteral("l"), QStringLiteral("lookup")},
                                    QStringLiteral("Look up <word> after starting."), QStringLiteral("word"));
    QCommandLineOption listDictionariesOption(QStringLiteral("list-dictionaries"),
                                              QStringLiteral("List the installed dictionaries and exit."));
    QCommandLineOption debugOption(QStringLiteral("debug"), QStringLiteral("Enable debug logging."));
    parser.addOptions({configOption, databaseOption, lookupOption, listDictionariesOption, debugOption});
    parser.process(app);

    Lexicon::Logging::initialize(parser.isSet(debugOption));

    Lexicon::ConfigLoader loader;
    Lexicon::ConfigLoadResult loaded = loader.load(parser.value(configOption));
    if (!loaded.found) {
        qCInfo(lexiconConfig) << "event=config_defaults path=" << loaded.sourcePath;
    }
    if (parser.isSet(databaseOption)) {
        loaded.config.dictionary.databasePath = parser.value(databaseOption);
    }

    if (parser.isSet(listDictionariesOption)) {
        const auto open = [](const QString& path) -> std::unique_ptr<Lexicon::DictionaryRepository> {
            return std::make_unique<Lexicon::SqliteDictionaryRepository>(path);
        };
        const Lexicon::DictionaryCatalog catalog(loaded.config.resolvedDictionaryDirectory(), open);

        QTextStream out(stdout);
        if (catalog.entries().isEmpty()) {
            out << "No dictionaries installed in " << catalog.directory() << '\n';
            return 0;
        }
        for (const Lexicon::DictionaryCatalogEntry& entry : catalog.entries()) {
            out << entry.info.name << "\t" << entry.info.languagePairLabel() << "\t" << entry.entryCount << " entries\t"
                << entry.path << '\n';
        }
        return 0;
    }

    Lexicon::Application application(loaded.config);
    return application.run(parser.value(lookupOption));
}

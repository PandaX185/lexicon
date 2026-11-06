// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import "settings"
import "themes"

ApplicationWindow {
    id: window

    width: 780
    height: 680
    minimumWidth: 640
    minimumHeight: 520
    visible: false
    title: qsTr("Lexicon Settings")
    color: theme.background

    Theme {
        id: theme
        // `system` is a supported TOML value, but palette probing is not
        // implemented yet. For now `system` behaves like `dark`.
        darkMode: config.themeUsesDarkPalette !== false
    }

    SettingsView {
        anchors.fill: parent
        darkMode: theme.darkMode
    }
}

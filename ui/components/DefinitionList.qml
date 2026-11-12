// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../themes"

ColumnLayout {
    id: root

    property var definitions: []

    spacing: 12

    Theme {
        id: theme
        // `system` follows the platform color scheme.
        darkMode: config.themeUsesDarkPalette !== false
    }

    Repeater {
        model: root.definitions

        delegate: ColumnLayout {
            Layout.fillWidth: true
            spacing: 2

            Label {
                text: modelData.partOfSpeech + (modelData.label ? " · " + modelData.label : "")
                color: theme.accent
                font.pixelSize: 11
                font.capitalization: Font.AllUppercase
                font.letterSpacing: 0.6
            }

            Label {
                Layout.fillWidth: true
                text: modelData.text
                color: theme.textPrimary
                wrapMode: Text.WordWrap
                font.pixelSize: 14
                lineHeight: 1.25
            }
        }
    }
}

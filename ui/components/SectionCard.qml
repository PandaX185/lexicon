// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../themes"

Rectangle {
    id: card

    property string title
    default property alias content: body.data

    implicitHeight: layout.implicitHeight + 32
    radius: theme.radius
    color: theme.surface
    border.width: 1
    border.color: theme.border

    Theme {
        id: theme
        // `system` is a supported TOML value, but palette probing is not
        // implemented yet. For now `system` behaves like `dark`.
        darkMode: config.themeUsesDarkPalette !== false
    }

    ColumnLayout {
        id: layout
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.margins: 16
        spacing: 12

        Label {
            text: card.title
            color: theme.textPrimary
            font.pixelSize: 16
            font.weight: Font.DemiBold
        }

        ColumnLayout {
            id: body
            Layout.fillWidth: true
            spacing: 8
        }
    }
}

// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window
import "../components"
import "../themes"

Item {
        id: root

        property int preferredWidth: 360
        property int animationDuration: 140
        property bool loading: false
        property string word: ""
        property var entry: null
        property bool hasMessage: false
        property string messageTitle: ""
        property string message: ""

        width: preferredWidth
        height: card.implicitHeight + 20

    Theme {
        id: theme
        darkMode: config.themeUsesDarkPalette !== false
    }
            MouseArea {
            anchors.fill: parent
            enabled: typeof config !== "undefined" && config.dismissOnOutsideClick !== false
            onClicked: Window.window.close()
        }


    Rectangle {
        id: card
        width: parent.width - 20
        implicitHeight: content.implicitHeight + 36
        anchors.centerIn: parent
        radius: theme.radius
        color: theme.surface
        border.width: 1
        border.color: theme.border
        opacity: 0

        Behavior on opacity {
            NumberAnimation {
                duration: root.animationDuration
                easing.type: Easing.OutCubic
            }
        }

        Component.onCompleted: opacity = 1

        ColumnLayout {
            id: content
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.margins: 18
            spacing: 12

            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Label {
                    Layout.fillWidth: true
                    text: root.word.length > 0 ? root.word : (root.hasMessage ? root.messageTitle : "Lexicon")
                    color: theme.textPrimary
                    font.pixelSize: 22
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                ToolButton {
                    text: qsTr("×")
                    font.pixelSize: 22
                    implicitWidth: 34
                    implicitHeight: 34
                    onClicked: Window.window.close()
                }
            }

            Rectangle {
                Layout.fillWidth: true
                implicitHeight: 1
                color: theme.border
            }

            BusyIndicator {
                visible: root.loading
                running: root.loading
                Layout.alignment: Qt.AlignHCenter
                Layout.preferredWidth: 32
                Layout.preferredHeight: 32
            }

            Label {
                visible: root.loading
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Looking up…")
                color: theme.textSecondary
            }

            ColumnLayout {
                visible: !root.loading && !root.hasMessage && !!root.entry
                Layout.fillWidth: true
                spacing: 10

                Label {
                    visible: root.entry !== null
                             && root.entry !== undefined
                             && root.entry.pronunciation !== undefined
                             && root.entry.pronunciation.respelling !== ""
                    text: root.entry && root.entry.pronunciation
                          ? root.entry.pronunciation.respelling : ""
                    color: theme.textSecondary
                    font.pixelSize: 13
                }

                ToolButton {
                    visible: root.entry !== null
                             && root.entry !== undefined
                             && root.entry.pronunciation !== undefined
                             && root.entry.pronunciation.respelling !== ""
                    text: qsTr("⊖")
                    font.pixelSize: 14
                    implicitWidth: 30
                    implicitHeight: 30
                    onClicked: {
                        if (root.entry && root.entry.pronunciation) {
                            let current = root.entry.pronunciation.respelling;
                            root.entry.pronunciation.respelling = current === "" ? "/" : "";
                        }
                    }
                }

                DefinitionList {
                    Layout.fillWidth: true
                    definitions: root.entry && root.entry.definitions
                                 ? root.entry.definitions : []
                }

                Label {
                    visible: root.entry !== null
                             && root.entry !== undefined
                             && root.entry.metadata !== undefined
                             && root.entry.metadata.source !== undefined
                    text: root.entry && root.entry.metadata && root.entry.metadata.source
                          ? qsTr("Source: %1").arg(root.entry.metadata.source) : ""
                    color: theme.textSecondary
                    font.pixelSize: 10
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
            }

            ColumnLayout {
                visible: !root.loading && root.hasMessage
                Layout.fillWidth: true
                spacing: 6

                Label {
                    Layout.fillWidth: true
                    text: root.messageTitle
                    color: theme.textPrimary
                    font.pixelSize: 16
                    font.weight: Font.DemiBold
                    wrapMode: Text.WordWrap
                }

                Label {
                    Layout.fillWidth: true
                    text: root.message
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 13
                    lineHeight: 1.25
                }
            }
        }
    }

    Shortcut {
        sequence: qsTr("Escape")
        onActivated: {
            if (typeof config !== "undefined" && config.dismissOnEscape === false) {
                return;
            }
            Window.window.close();
        }
    }
}

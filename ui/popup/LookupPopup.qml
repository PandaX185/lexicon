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

    // Bounds of the screen the popup opens on, pushed in by QmlPresenter before
    // the popup is shown. `0` means "unconstrained". `autoPopupHeight` is the
    // height the popup may grow to on its own; the resize grip may exceed it up
    // to `maxPopupHeight`.
    property int maxPopupWidth: 0
    property int maxPopupHeight: 0
    property int autoPopupHeight: 0

    // The popup follows its content until the user drags the resize grip; after
    // that the chosen size is reused for later lookups.
    property bool userResized: false
    property real manualWidth: 0
    property real manualHeight: 0

    readonly property int minimumPopupWidth: 240
    readonly property int minimumPopupHeight: 120
    readonly property real naturalHeight: card.implicitHeight + 20

    function clampWidth(candidate) {
        const upper = root.maxPopupWidth > 0 ? Math.max(root.maxPopupWidth, root.minimumPopupWidth) : candidate;
        return Math.min(Math.max(candidate, root.minimumPopupWidth), upper);
    }

    function clampHeight(candidate, limit) {
        const upper = limit > 0 ? Math.max(limit, root.minimumPopupHeight) : candidate;
        return Math.min(Math.max(candidate, root.minimumPopupHeight), upper);
    }

    width: root.userResized ? root.clampWidth(root.manualWidth) : root.clampWidth(root.preferredWidth)
    height: root.userResized
            ? root.clampHeight(root.manualHeight, root.maxPopupHeight)
            : root.clampHeight(root.naturalHeight, root.autoPopupHeight)

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
        height: parent.height - 20
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
            anchors.fill: parent
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
                    wrapMode: Text.Wrap
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

            // Long entries scroll instead of pushing the popup off screen; the
            // header above stays visible so the close button is always reachable.
            ScrollView {
                id: bodyScroll
                Layout.fillWidth: true
                Layout.fillHeight: true
                contentWidth: availableWidth
                clip: true

                ColumnLayout {
                    id: body
                    width: bodyScroll.availableWidth
                    spacing: 12

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
                            wrapMode: Text.Wrap
                            Layout.fillWidth: true
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
                            wrapMode: Text.Wrap
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
                            wrapMode: Text.Wrap
                        }

                        Label {
                            Layout.fillWidth: true
                            text: root.message
                            color: theme.textSecondary
                            wrapMode: Text.Wrap
                            font.pixelSize: 13
                            lineHeight: 1.25
                        }
                    }
                }
            }
        }

        MouseArea {
            id: resizeGrip
            width: 18
            height: 18
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            cursorShape: Qt.SizeFDiagCursor
            hoverEnabled: true

            property point pressPoint: Qt.point(0, 0)
            property real startWidth: 0
            property real startHeight: 0

            onPressed: (mouse) => {
                pressPoint = Qt.point(mouse.x, mouse.y);
                startWidth = root.width;
                startHeight = root.height;
                root.userResized = true;
                root.manualWidth = root.width;
                root.manualHeight = root.height;
            }

            onPositionChanged: (mouse) => {
                if (!pressed) {
                    return;
                }
                root.manualWidth = startWidth + (mouse.x - pressPoint.x);
                root.manualHeight = startHeight + (mouse.y - pressPoint.y);
            }

            Rectangle {
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: 3
                width: 9
                height: 9
                radius: 2
                color: theme.border
                opacity: resizeGrip.containsMouse ? 1.0 : 0.6
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

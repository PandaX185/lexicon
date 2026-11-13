// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"
import "../themes"

Item {
    id: root

    property bool darkMode: config.themeUsesDarkPalette !== false

    Theme {
        id: theme
        darkMode: root.darkMode
    }

    ScrollView {
        anchors.fill: parent
        contentWidth: availableWidth
        clip: true

        ColumnLayout {
            width: Math.min(720, root.width - 48)
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 16

            Label {
                Layout.topMargin: 32
                text: qsTr("Lexicon")
                color: theme.textPrimary
                font.pixelSize: 30
                font.weight: Font.DemiBold
            }

            Label {
                text: qsTr("Lightweight offline English dictionary for KDE Plasma")
                color: theme.textSecondary
                font.pixelSize: 14
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Status")

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Dictionary: %1").arg(config.databasePath)
                    color: theme.textPrimary
                    wrapMode: Text.WordWrap
                    font.family: "monospace"
                    font.pixelSize: 12
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Global shortcut: %1").arg(settings.lookup)
                    color: theme.textPrimary
                    font.pixelSize: 13
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Interaction")

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Dismiss on Escape"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch { checked: settings.dismissOnEscape; onToggled: settings.dismissOnEscape = checked }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Dismiss on outside click"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch { checked: settings.dismissOnOutsideClick; onToggled: settings.dismissOnOutsideClick = checked }
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Appearance")

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Theme"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        model: ["dark", "light", "system"]
                        currentIndex: Math.max(0, model.indexOf(settings.theme))
                        onActivated: settings.theme = model[currentIndex]
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Popup width"); color: theme.textPrimary; Layout.fillWidth: true }
                    SpinBox { from: 240; to: 1200; value: settings.popupWidth; onValueChanged: settings.popupWidth = value }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Animation"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        model: ["none", "subtle"]
                        currentIndex: Math.max(0, model.indexOf(settings.animation))
                        onActivated: settings.animation = model[currentIndex]
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Position"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        model: ["near_selection", "center", "cursor"]
                        currentIndex: Math.max(0, model.indexOf(settings.position))
                        onActivated: settings.position = model[currentIndex]
                    }
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Language packs & translation")

                Label {
                    Layout.fillWidth: true
                    text: languagePacks.status
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 12
                }

                Repeater {
                    model: languagePacks.installedPairs
                    delegate: RowLayout {
                        Layout.fillWidth: true
                        spacing: 8
                        Label {
                            Layout.fillWidth: true
                            text: modelData.label
                            color: theme.textPrimary
                            font.pixelSize: 13
                        }
                        Button {
                            text: qsTr("Remove")
                            enabled: !languagePacks.busy
                            onClicked: languagePacks.remove(modelData.source, modelData.target)
                        }
                    }
                }

                Label {
                    Layout.fillWidth: true
                    visible: languagePacks.installedPairs.length === 0
                    text: qsTr("No translation pairs are installed.")
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 12
                }

                Repeater {
                    model: languagePacks.availablePacks
                    delegate: ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 4
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8
                            Label {
                                Layout.fillWidth: true
                                text: modelData.label !== undefined ? modelData.label
                                      : qsTr("%1 → %2").arg(modelData.source, modelData.target)
                                color: theme.textPrimary
                                font.pixelSize: 13
                                wrapMode: Text.WordWrap
                            }
                            Button {
                                text: qsTr("Install")
                                enabled: !languagePacks.busy
                                onClicked: languagePacks.install(modelData.id)
                            }
                        }
                        ProgressBar {
                            Layout.fillWidth: true
                            visible: languagePacks.activePack === modelData.id && languagePacks.busy
                            from: 0; to: 1; value: languagePacks.progress
                        }
                    }
                }

                Button {
                    text: qsTr("Refresh catalog")
                    enabled: languagePacks.catalogConfigured && !languagePacks.busy
                    onClicked: languagePacks.refresh()
                }

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Catalog URL"); color: theme.textPrimary; Layout.fillWidth: true }
                    TextField {
                        Layout.fillWidth: true
                        text: settings.catalogUrl
                        onEditingFinished: settings.catalogUrl = text
                    }
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Dictionary")

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Show pronunciation"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch { checked: settings.showPronunciation; onToggled: settings.showPronunciation = checked }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Pronunciation style"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        model: ["respelling", "ipa"]
                        currentIndex: Math.max(0, model.indexOf(settings.pronunciationStyle))
                        onActivated: settings.pronunciationStyle = model[currentIndex]
                    }
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Startup and performance")

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Start on login"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch { checked: settings.startOnLogin; onToggled: settings.startOnLogin = checked }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Lookup cache"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch { checked: settings.cacheEnabled; onToggled: settings.cacheEnabled = checked }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Cache size"); color: theme.textPrimary; Layout.fillWidth: true }
                    SpinBox { from: 0; to: 4096; value: settings.cacheSize; onValueChanged: settings.cacheSize = value }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                Button {
                    text: qsTr("Save")
                    onClicked: settings.save()
                }
                Label {
                    Layout.fillWidth: true
                    text: settings.status
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 12
                }
            }

            Label {
                Layout.fillWidth: true
                Layout.bottomMargin: 32
                text: qsTr("Lexicon %1").arg(appVersion)
                color: theme.textSecondary
                font.pixelSize: 11
            }
        }
    }
}

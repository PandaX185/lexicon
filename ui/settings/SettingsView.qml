// qmllint disable unqualified
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "../components"
import "../themes"

FocusScope {
    id: root
    objectName: "settingsRoot"

    property bool darkMode: config.themeUsesDarkPalette !== false

    // The settings content is a single focus scope so Tab and Shift+Tab move
    // between the controls in declaration order, and the window starts with
    // focus inside the view.
    focus: true
    Accessible.role: Accessible.Pane
    Accessible.name: qsTr("Lexicon settings")
    Accessible.description: qsTr("Configure interaction, appearance, dictionaries, and language packs")

    AppTheme {
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
                    Switch {
                        objectName: "dismissEscapeSwitch"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Dismiss on Escape")
                        checked: settings.dismissOnEscape
                        onToggled: settings.dismissOnEscape = checked
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Dismiss on outside click"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch {
                        objectName: "dismissOutsideSwitch"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Dismiss on outside click")
                        checked: settings.dismissOnOutsideClick
                        onToggled: settings.dismissOnOutsideClick = checked
                    }
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Appearance")

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Theme"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        objectName: "themeCombo"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Theme")
                        model: ["dark", "light", "system"]
                        currentIndex: Math.max(0, model.indexOf(settings.theme))
                        onActivated: settings.theme = model[currentIndex]
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Popup width"); color: theme.textPrimary; Layout.fillWidth: true }
                    SpinBox {
                        objectName: "popupWidthSpin"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Popup width")
                        from: 240
                        to: 1200
                        value: settings.popupWidth
                        onValueChanged: settings.popupWidth = value
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Animation"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        objectName: "animationCombo"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Animation")
                        model: ["none", "subtle"]
                        currentIndex: Math.max(0, model.indexOf(settings.animation))
                        onActivated: settings.animation = model[currentIndex]
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Position"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        objectName: "positionCombo"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Popup position")
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
                            activeFocusOnTab: true
                            Accessible.name: qsTr("Remove %1").arg(modelData.label)
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
                                activeFocusOnTab: true
                                Accessible.name: qsTr("Install %1")
                                            .arg(modelData.label !== undefined ? modelData.label
                                                                               : qsTr("%1 to %2").arg(modelData.source,
                                                                                                      modelData.target))
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
                    objectName: "refreshCatalogButton"
                    text: qsTr("Refresh catalog")
                    activeFocusOnTab: true
                    Accessible.name: qsTr("Refresh language pack catalog")
                    enabled: languagePacks.catalogConfigured && !languagePacks.busy
                    onClicked: languagePacks.refresh()
                }

                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Catalog URL"); color: theme.textPrimary; Layout.fillWidth: true }
                    TextField {
                        objectName: "catalogUrlField"
                        Layout.fillWidth: true
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Language pack catalog URL")
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
                    Switch {
                        objectName: "showPronunciationSwitch"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Show pronunciation")
                        checked: settings.showPronunciation
                        onToggled: settings.showPronunciation = checked
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Pronunciation style"); color: theme.textPrimary; Layout.fillWidth: true }
                    ComboBox {
                        objectName: "pronunciationStyleCombo"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Pronunciation style")
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
                    Switch {
                        objectName: "startOnLoginSwitch"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Start on login")
                        checked: settings.startOnLogin
                        onToggled: settings.startOnLogin = checked
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Lookup cache"); color: theme.textPrimary; Layout.fillWidth: true }
                    Switch {
                        objectName: "cacheEnabledSwitch"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Lookup cache")
                        checked: settings.cacheEnabled
                        onToggled: settings.cacheEnabled = checked
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Label { text: qsTr("Cache size"); color: theme.textPrimary; Layout.fillWidth: true }
                    SpinBox {
                        objectName: "cacheSizeSpin"
                        activeFocusOnTab: true
                        Accessible.name: qsTr("Cache size")
                        from: 0
                        to: 4096
                        value: settings.cacheSize
                        onValueChanged: settings.cacheSize = value
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                Button {
                    objectName: "saveButton"
                    text: qsTr("Save")
                    activeFocusOnTab: true
                    Accessible.name: qsTr("Save settings")
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

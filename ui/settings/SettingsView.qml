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

            Label {
                Layout.topMargin: 8
                text: qsTr("Most settings are read-only in Stage 1 and take effect after restart.")
                color: theme.textSecondary
                font.pixelSize: 12
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
                    text: qsTr("Global shortcut: %1").arg(config.lookup)
                    color: theme.textPrimary
                    font.pixelSize: 13
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Selection lookup: not available yet. Only the documented shortcut and command-line lookup are active.")
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 12
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Interaction")

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Activation: %1").arg(config.activation)
                    color: theme.textPrimary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Clipboard fallback: %1 (not used in Stage 1)").arg(config.clipboardFallback ? qsTr("enabled") : qsTr("disabled"))
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Dismiss on Escape: %1 (while the popup has focus)").arg(config.dismissOnEscape ? qsTr("yes") : qsTr("no"))
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Dismiss on outside click: %1 (inside popup window only)").arg(config.dismissOnOutsideClick ? qsTr("yes") : qsTr("no"))
                    color: theme.textSecondary
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Appearance")

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Theme: %1").arg(config.theme)
                    color: theme.textPrimary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Popup width: %1 px").arg(config.popupWidth)
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Popup position: %1").arg(config.position)
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Animation: %1").arg(config.animation)
                    color: theme.textSecondary
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

                // Installed packs: translation is offline once a pair is here.
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
                    text: qsTr("No translation pairs are installed. Mixed-language lookups show the dictionary entry only.")
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 12
                }

                // Downloads only ever start from these buttons.
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
                                text: modelData.label !== undefined
                                      ? modelData.label
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

                        Label {
                            Layout.fillWidth: true
                            visible: modelData.description !== ""
                            text: modelData.description
                            color: theme.textSecondary
                            wrapMode: Text.WordWrap
                            font.pixelSize: 11
                        }

                        ProgressBar {
                            Layout.fillWidth: true
                            visible: languagePacks.activePack === modelData.id
                                     && languagePacks.busy
                            from: 0
                            to: 1
                            value: languagePacks.progress
                        }
                    }
                }

                Button {
                    text: qsTr("Refresh catalog")
                    enabled: languagePacks.catalogConfigured && !languagePacks.busy
                    onClicked: languagePacks.refresh()
                }

                Label {
                    Layout.fillWidth: true
                    visible: !languagePacks.catalogConfigured
                    text: qsTr("No language pack catalog is configured. Set translation.catalog_url in the configuration file to enable downloads.")
                    color: theme.textSecondary
                    wrapMode: Text.WordWrap
                    font.pixelSize: 11
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Dictionary")

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Show pronunciation: %1").arg(config.showPronunciation ? qsTr("yes") : qsTr("no"))
                    color: theme.textPrimary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Pronunciation style: %1").arg(config.pronunciationStyle)
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Offline first: %1 (reserved)").arg(config.offlineFirst ? qsTr("yes") : qsTr("no"))
                    color: theme.textSecondary
                }
            }

            SectionCard {
                Layout.fillWidth: true
                title: qsTr("Startup and performance")

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Start on login: %1").arg(config.startOnLogin ? qsTr("yes") : qsTr("no"))
                    color: theme.textPrimary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Lazy initialization: %1 (popup appears on lookup)").arg(config.lazyInitialize ? qsTr("yes") : qsTr("no"))
                    color: theme.textSecondary
                }

                Label {
                    Layout.fillWidth: true
                    text: qsTr("Lookup cache: %1 (not used in Stage 1)").arg(config.cacheEnabled ? qsTr("enabled") : qsTr("disabled"))
                    color: theme.textSecondary
                }
            }

            Label {
                Layout.fillWidth: true
                Layout.topMargin: 4
                text: qsTr("This window is read-only in the Stage 1 scaffold. Edit ~/.config/lexicon/config.toml and restart Lexicon to change settings. Some options are reserved for future stages.")
                color: theme.textSecondary
                wrapMode: Text.WordWrap
                font.pixelSize: 11
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

// qmllint disable unqualified
import QtQuick

Theme {
    id: root

    readonly property var appConfig: typeof config !== "undefined" && config !== null ? config : ({})

    darkMode: root.appConfig.themeUsesDarkPalette !== false
    colorOverrides: root.appConfig.themeColors !== undefined ? root.appConfig.themeColors : ({})
    cornerRadius: root.appConfig.cornerRadius !== undefined ? root.appConfig.cornerRadius : 14
}

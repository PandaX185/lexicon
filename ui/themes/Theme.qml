import QtQuick

QtObject {
    property bool darkMode: true

    readonly property color background: darkMode ? "#101218" : "#f4f5f8"
    readonly property color surface: darkMode ? "#191c24" : "#ffffff"
    readonly property color surfaceRaised: darkMode ? "#222632" : "#eceef4"
    readonly property color border: darkMode ? "#303646" : "#d8dbe4"
    readonly property color textPrimary: darkMode ? "#f2f4f8" : "#171a21"
    readonly property color textSecondary: darkMode ? "#a5adbe" : "#5b6271"
    readonly property color accent: darkMode ? "#7aa2f7" : "#2563eb"
    readonly property color danger: "#f7768e"
    readonly property int radius: 14

    // `system` is a supported TOML value, but the palette does not yet probe the
    // desktop theme. For now `system` behaves like `dark`.
}

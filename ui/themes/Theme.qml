import QtQuick

QtObject {
    property bool darkMode: true
    // Optional per-token overrides, typically the `[appearance.colors]` table
    // from config.toml. Missing keys fall back to the built-in palette.
    property var colorOverrides: ({})
    property int cornerRadius: 14

    readonly property color background: colorOverrides.background !== undefined ? colorOverrides.background
                                                                              : (darkMode ? "#101218" : "#f4f5f8")
    readonly property color surface: colorOverrides.surface !== undefined ? colorOverrides.surface
                                                                          : (darkMode ? "#191c24" : "#ffffff")
    readonly property color surfaceRaised: colorOverrides.surfaceRaised !== undefined ? colorOverrides.surfaceRaised
                                                                                      : (darkMode ? "#222632" : "#eceef4")
    readonly property color border: colorOverrides.border !== undefined ? colorOverrides.border
                                                                        : (darkMode ? "#303646" : "#d8dbe4")
    readonly property color textPrimary: colorOverrides.textPrimary !== undefined ? colorOverrides.textPrimary
                                                                                  : (darkMode ? "#f2f4f8" : "#171a21")
    readonly property color textSecondary: colorOverrides.textSecondary !== undefined ? colorOverrides.textSecondary
                                                                                      : (darkMode ? "#a5adbe" : "#5b6271")
    readonly property color accent: colorOverrides.accent !== undefined ? colorOverrides.accent
                                                                        : (darkMode ? "#7aa2f7" : "#2563eb")
    readonly property color danger: colorOverrides.danger !== undefined ? colorOverrides.danger : "#f7768e"
    readonly property int radius: cornerRadius

    // `system` is resolved by Config::toVariantMap from the platform color scheme.
}

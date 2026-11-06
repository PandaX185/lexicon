add_library(lexicon_kde STATIC
    shortcuts/KGlobalAccelShortcutService.cpp
    selection/KdeSelectionProvider.cpp
    tray/KStatusNotifierTrayService.cpp
    session/KdeSession.cpp
)

target_include_directories(lexicon_kde PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_kde PUBLIC
    lexicon_core
    lexicon_config
    lexicon_infrastructure
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::DBus
    KF6::GlobalAccel
    KF6::StatusNotifierItem
    KF6::DBusAddons
)
target_compile_features(lexicon_kde PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_kde)

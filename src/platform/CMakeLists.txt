# Platform backends are optional adapters. Each one implements a core
# interface (currently PrimarySelectionSource) and nothing above the
# composition root includes it directly, so backends can be added, removed, or
# swapped without touching core, the controller, or the UI.
if(LEXICON_WITH_WAYLAND)
    add_subdirectory(wayland)
endif()

if(TARGET lexicon_wayland)
    message(STATUS "Selection backends: Wayland (ext-data-control-v1)")
else()
    message(STATUS "Selection backends: none (selection retrieval reports an explicit error)")
endif()

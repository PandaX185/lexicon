add_library(lexicon_controller STATIC
    ApplicationController.cpp
)

target_include_directories(lexicon_controller PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_controller PUBLIC
    lexicon_config
    lexicon_dictionary
    lexicon_core
    Qt6::Core
    Qt6::Concurrent
)
target_compile_features(lexicon_controller PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_controller)

add_library(lexicon_app STATIC
    Application.cpp
    QmlPresenter.cpp
    SelectionProviderFactory.cpp
)

target_include_directories(lexicon_app PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_app PUBLIC
    lexicon_controller
    lexicon_kde
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::Qml
    Qt6::Quick
    Qt6::QuickControls2
)

# The composition root is the only place that knows which platform backends
# exist. Backends are optional, so it gets a compile definition per available
# backend and selects among them at runtime.
if(TARGET lexicon_wayland)
    target_link_libraries(lexicon_app PUBLIC lexicon_wayland)
    target_compile_definitions(lexicon_app PRIVATE LEXICON_HAVE_WAYLAND=1)
endif()

target_compile_features(lexicon_app PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_app)

add_executable(lexicon main.cpp)

target_link_libraries(lexicon PRIVATE
    lexicon_app
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
)
target_compile_features(lexicon PRIVATE cxx_std_20)
lexicon_enable_warnings(lexicon)

set_target_properties(lexicon PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}
)

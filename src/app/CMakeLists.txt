add_executable(lexicon
    main.cpp
    Application.cpp
    ApplicationController.cpp
    QmlPresenter.cpp
)

target_include_directories(lexicon PRIVATE ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon PRIVATE
    lexicon_core
    lexicon_config
    lexicon_dictionary
    lexicon_infrastructure
    lexicon_kde
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::Qml
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::Concurrent
)
target_compile_features(lexicon PRIVATE cxx_std_20)
lexicon_enable_warnings(lexicon)

set_target_properties(lexicon PROPERTIES
    RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}
)

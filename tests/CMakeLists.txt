function(lexicon_add_test name source label)
    add_executable(${name} ${source})
    target_include_directories(${name} PRIVATE ${CMAKE_SOURCE_DIR}/src ${CMAKE_SOURCE_DIR}/tests)
    target_link_libraries(${name} PRIVATE Qt6::Test ${ARGN})
    target_compile_features(${name} PRIVATE cxx_std_20)
    target_compile_definitions(${name} PRIVATE LEXICON_SOURCE_DIR="${CMAKE_SOURCE_DIR}")
    lexicon_enable_warnings(${name})

    add_test(NAME ${name} COMMAND ${name})
    set_tests_properties(${name} PROPERTIES LABELS "${label}")
endfunction()

lexicon_add_test(test_core_types unit/test_core_types.cpp unit lexicon_dictionary)
lexicon_add_test(test_config unit/test_config.cpp unit lexicon_config)
lexicon_add_test(test_dictionary_service unit/test_dictionary_service.cpp unit lexicon_dictionary)
lexicon_add_test(test_application_controller unit/test_application_controller.cpp unit
    lexicon_controller
    Qt6::Concurrent
)
lexicon_add_test(test_kde_selection_provider unit/test_kde_selection_provider.cpp unit lexicon_kde)
lexicon_add_test(test_sqlite_repository integration/test_sqlite_repository.cpp integration
    lexicon_infrastructure
    Qt6::Sql
    Qt6::Concurrent
)
lexicon_add_test(test_qml_views integration/test_qml_views.cpp qml
    lexicon_config
    Qt6::Gui
    Qt6::Qml
    Qt6::Quick
    Qt6::QuickControls2
)
set_tests_properties(test_qml_views PROPERTIES
    ENVIRONMENT "QT_QPA_PLATFORM=offscreen;QT_QUICK_BACKEND=software"
)

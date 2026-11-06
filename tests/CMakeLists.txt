function(lexicon_add_test name source label)
    add_executable(${name} ${source})
    target_include_directories(${name} PRIVATE ${CMAKE_SOURCE_DIR}/src ${CMAKE_SOURCE_DIR}/tests)
    target_link_libraries(${name} PRIVATE Qt6::Test ${ARGN})
    target_compile_features(${name} PRIVATE cxx_std_20)
    lexicon_enable_warnings(${name})

    add_test(NAME ${name} COMMAND ${name})
    set_tests_properties(${name} PROPERTIES LABELS "${label}")
endfunction()

lexicon_add_test(test_config unit/test_config.cpp unit lexicon_config)
lexicon_add_test(test_dictionary_service unit/test_dictionary_service.cpp unit lexicon_dictionary)
lexicon_add_test(test_sqlite_repository integration/test_sqlite_repository.cpp integration
    lexicon_infrastructure
    Qt6::Sql
)
target_compile_definitions(test_sqlite_repository PRIVATE
    LEXICON_SOURCE_DIR="${CMAKE_SOURCE_DIR}"
)

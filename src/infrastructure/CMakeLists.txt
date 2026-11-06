add_library(lexicon_logging STATIC
    logging/Logging.cpp
)

target_include_directories(lexicon_logging PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_logging PUBLIC lexicon_core Qt6::Core)
target_compile_features(lexicon_logging PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_logging)

add_library(lexicon_infrastructure STATIC
    database/SqliteDictionaryRepository.cpp
    UnixSignalHandler.cpp
)

target_include_directories(lexicon_infrastructure PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_infrastructure PUBLIC
    lexicon_core
    lexicon_dictionary
    lexicon_logging
    Qt6::Core
    Qt6::Sql
)
target_compile_features(lexicon_infrastructure PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_infrastructure)

add_library(lexicon_config STATIC
    Config.cpp
    ConfigLoader.cpp
)

target_include_directories(lexicon_config PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_config PUBLIC
    lexicon_core
    lexicon_logging
    Qt6::Core
    Qt6::Gui
    ${LEXICON_TOML_TARGET}
)
target_compile_features(lexicon_config PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_config)

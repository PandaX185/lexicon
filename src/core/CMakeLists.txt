add_library(lexicon_core STATIC
    errors/AppError.cpp
    selection/PrimarySelectionProvider.cpp
    selection/SelectionTextPolicy.cpp
    selection/UnsupportedPrimarySelectionSource.cpp
    text/LookupText.cpp
)

target_include_directories(lexicon_core PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_core PUBLIC Qt6::Core)
target_compile_features(lexicon_core PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_core)

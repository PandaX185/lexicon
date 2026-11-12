add_library(lexicon_dictionary STATIC
    models/PartOfSpeech.cpp
    DictionaryService.cpp
    DictionaryCatalog.cpp
    CatalogDictionaryRepository.cpp
    ArpabetToIpa.cpp
)

target_include_directories(lexicon_dictionary PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_link_libraries(lexicon_dictionary PUBLIC lexicon_core Qt6::Core)
target_compile_features(lexicon_dictionary PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_dictionary)

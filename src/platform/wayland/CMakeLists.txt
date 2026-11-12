# The Wayland backend is an optional adapter. When the toolchain or protocol
# dependencies are missing we skip the target instead of failing the whole
# configure, because nothing outside this directory depends on it.
find_package(PkgConfig QUIET)
if(NOT PkgConfig_FOUND)
    message(WARNING "pkg-config not found; the Wayland selection backend will not be built")
    return()
endif()

pkg_check_modules(WAYLAND_CLIENT QUIET IMPORTED_TARGET wayland-client)
find_program(WAYLAND_SCANNER_EXECUTABLE NAMES wayland-scanner)

if(NOT WAYLAND_CLIENT_FOUND)
    message(WARNING "wayland-client not found; the Wayland selection backend will not be built")
    return()
endif()

if(NOT WAYLAND_SCANNER_EXECUTABLE)
    message(WARNING "wayland-scanner not found; the Wayland selection backend will not be built")
    return()
endif()

set(EXT_DATA_CONTROL_XML
    ${CMAKE_SOURCE_DIR}/third_party/wayland-protocols/ext-data-control/ext-data-control-v1.xml)
set(EXT_DATA_CONTROL_GENERATED_DIR ${CMAKE_CURRENT_BINARY_DIR}/ext-data-control)
set(EXT_DATA_CONTROL_CLIENT_HEADER
    ${EXT_DATA_CONTROL_GENERATED_DIR}/ext-data-control-v1-client-protocol.h)
set(EXT_DATA_CONTROL_PROTOCOL_SOURCE
    ${EXT_DATA_CONTROL_GENERATED_DIR}/ext-data-control-v1-protocol.c)

add_custom_command(
    OUTPUT
        ${EXT_DATA_CONTROL_CLIENT_HEADER}
        ${EXT_DATA_CONTROL_PROTOCOL_SOURCE}
    COMMAND ${CMAKE_COMMAND} -E make_directory ${EXT_DATA_CONTROL_GENERATED_DIR}
    COMMAND ${WAYLAND_SCANNER_EXECUTABLE} client-header
        ${EXT_DATA_CONTROL_XML} ${EXT_DATA_CONTROL_CLIENT_HEADER}
    COMMAND ${WAYLAND_SCANNER_EXECUTABLE} private-code
        ${EXT_DATA_CONTROL_XML} ${EXT_DATA_CONTROL_PROTOCOL_SOURCE}
    DEPENDS ${EXT_DATA_CONTROL_XML}
    COMMENT "Generating ext-data-control-v1 protocol bindings"
    VERBATIM
)

add_library(lexicon_wayland STATIC
    WaylandDataControlSource.cpp
    ${EXT_DATA_CONTROL_PROTOCOL_SOURCE}
)

target_include_directories(lexicon_wayland PUBLIC ${CMAKE_SOURCE_DIR}/src)
target_include_directories(lexicon_wayland SYSTEM PRIVATE ${EXT_DATA_CONTROL_GENERATED_DIR})
target_link_libraries(lexicon_wayland PUBLIC
    lexicon_core
    Qt6::Core
    PkgConfig::WAYLAND_CLIENT
)
target_compile_features(lexicon_wayland PUBLIC cxx_std_20)
lexicon_enable_warnings(lexicon_wayland)

# Generated protocol code is not ours to lint or to make warnings-as-errors.
set_source_files_properties(${EXT_DATA_CONTROL_PROTOCOL_SOURCE}
    PROPERTIES COMPILE_OPTIONS "-w")

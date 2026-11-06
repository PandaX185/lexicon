cmake_minimum_required(VERSION 3.24)

project(Lexicon
    VERSION 0.1.0
    DESCRIPTION "Lightweight offline English dictionary utility for KDE Plasma"
    HOMEPAGE_URL "https://github.com/PandaX185/lexicon"
    LANGUAGES CXX
)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

set(LEXICON_APP_ID "org.lexicon.Lexicon")
set(LEXICON_COMPONENT_NAME "lexicon")

find_package(ECM 6.0 REQUIRED NO_MODULE)
set(CMAKE_MODULE_PATH ${ECM_MODULE_PATH} ${CMAKE_MODULE_PATH})
include(KDEInstallDirs6)
include(KDECompilerSettings NO_POLICY_SCOPE)
include(ECMEnableSanitizers)

find_package(Qt6 6.10 REQUIRED COMPONENTS
    Core
    Gui
    Qml
    Quick
    QuickControls2
    Sql
    Test
    DBus
    Concurrent
)
find_package(KF6 6.0 REQUIRED COMPONENTS
    GlobalAccel
    StatusNotifierItem
    DBusAddons
)

find_package(tomlplusplus 3.4 QUIET)
if(tomlplusplus_FOUND)
    set(LEXICON_TOML_TARGET tomlplusplus::tomlplusplus)
    message(STATUS "Using system tomlplusplus ${tomlplusplus_VERSION}")
else()
    add_library(lexicon_tomlplusplus INTERFACE)
    target_include_directories(lexicon_tomlplusplus SYSTEM INTERFACE ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    set(LEXICON_TOML_TARGET lexicon_tomlplusplus)
    message(STATUS "Using vendored toml++ 3.4.0 (MIT)")
endif()

qt_standard_project_setup(REQUIRES 6.10)
enable_testing()

option(LEXICON_ENABLE_SANITIZERS "Build with AddressSanitizer and UndefinedBehaviorSanitizer" OFF)
option(LEXICON_WARNINGS_AS_ERRORS "Treat compiler warnings as errors" OFF)

if(LEXICON_ENABLE_SANITIZERS)
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
        add_link_options(-fsanitize=address,undefined)
    else()
        message(FATAL_ERROR "Sanitizers are only supported with GCC or Clang")
    endif()
endif()

function(lexicon_enable_warnings target)
    if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
        target_compile_options(${target} PRIVATE
            -Wall
            -Wextra
            -Wpedantic
            -Wshadow
            -Wnon-virtual-dtor
            -Wold-style-cast
            -Wcast-align
            -Wunused
        )
        if(LEXICON_WARNINGS_AS_ERRORS)
            target_compile_options(${target} PRIVATE -Werror)
        endif()
    endif()
endfunction()

add_subdirectory(src)

qt_add_resources(lexicon "lexicon_resources"
    PREFIX "/"
    FILES
        ui/Main.qml
        ui/popup/LookupPopup.qml
        ui/settings/SettingsView.qml
        ui/components/DefinitionList.qml
        ui/components/SectionCard.qml
        ui/themes/Theme.qml
        resources/icons/lexicon.svg
)

add_subdirectory(tests)

include(CTest)

install(TARGETS lexicon RUNTIME DESTINATION ${KDE_INSTALL_BINDIR})
install(FILES packaging/shared/${LEXICON_APP_ID}.desktop DESTINATION ${KDE_INSTALL_APPDIR})
install(FILES resources/icons/lexicon.svg DESTINATION ${KDE_INSTALL_ICONDIR}/hicolor/scalable/apps)
install(FILES packaging/shared/${LEXICON_APP_ID}.metainfo.xml DESTINATION ${KDE_INSTALL_METAINFODIR})
install(FILES resources/config/config.example.toml DESTINATION ${KDE_INSTALL_DATADIR}/lexicon)
install(FILES data/dictionaries/schema.sql DESTINATION ${KDE_INSTALL_DATADIR}/lexicon/dictionaries)
install(FILES packaging/shared/lexicon-autostart.desktop
        DESTINATION ${KDE_INSTALL_APPDATA_DIR}/autostart
        OPTIONAL)
install(FILES README.md LICENSE CHANGELOG.md ROADMAP.md CONTRIBUTING.md SECURITY.md SUPPORT.md
    DESTINATION ${KDE_INSTALL_DATADIR}/doc/lexicon
)
install(DIRECTORY docs/ DESTINATION ${KDE_INSTALL_DATADIR}/doc/lexicon/docs)

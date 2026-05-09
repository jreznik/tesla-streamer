cmake_minimum_required(VERSION 3.10)
project(tesla-streamer-gui)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

find_package(Qt6 COMPONENTS Widgets Network DBus REQUIRED)

add_executable(tesla-streamer-gui
    src/main.cpp
    src/MainWindow.cpp
    src/INetworkManager.cpp
    src/NetworkManagerLinux.cpp
    src/IFirewallManager.cpp
    src/FirewallManagerLinux.cpp
    resources.qrc
)

target_link_libraries(tesla-streamer-gui Qt6::Widgets Qt6::Network Qt6::DBus)

install(TARGETS tesla-streamer-gui DESTINATION bin)

install(FILES io.github.jreznik.TeslaStreamer.desktop DESTINATION share/applications)
install(FILES io.github.jreznik.TeslaStreamer.metainfo.xml DESTINATION share/metainfo)
install(FILES io.github.jreznik.TeslaStreamer.svg DESTINATION share/icons/hicolor/scalable/apps)



cmake_minimum_required(VERSION 3.10)
project(koven VERSION 1.0.0 LANGUAGES C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

add_compile_options(-Wall -Wextra -Wpedantic)

find_package(eclipse-paho-mqtt-c REQUIRED)

set(SOURCES
    main.c
    koven.c
    mqtt_client.c
    protocol.c
)

add_executable(koven ${SOURCES})

target_link_libraries(koven eclipse-paho-mqtt-c::paho-mqtt3c)

enable_testing()

add_library(unity STATIC
    external/unity.c
)

target_include_directories(unity PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/external
)

add_executable(test_protocol
    tests/test_protocol.c
    protocol.c
)

target_link_libraries(test_protocol unity)

target_include_directories(test_protocol PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}
)

add_test(NAME protocol_tests COMMAND test_protocol)

add_executable(test_koven
    tests/test_koven.c
    koven.c
)

target_link_libraries(test_koven unity)

target_include_directories(test_koven PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}
)

add_test(NAME koven_tests COMMAND test_koven)

# Minimum CMake version required
cmake_minimum_required(VERSION 3.12)

# Project name and version
project(MukhosthoDb VERSION 1.0 LANGUAGES CXX)

# Specify the C++ standard
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Add the library target
add_library(mukhostho_db
    src/mukhostho_db.cpp
)

# Include the header files
target_include_directories(mukhostho_db
    PUBLIC
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
        $<INSTALL_INTERFACE:include>
)

# Optional: Add an alias target for convenience
add_library(MukhosthoDb::MukhosthoDb ALIAS mukhostho_db)

# Install rules
install(TARGETS mukhostho_db
    EXPORT MukhosthoDbTargets
    LIBRARY DESTINATION lib
    ARCHIVE DESTINATION lib
    RUNTIME DESTINATION bin
    INCLUDES DESTINATION include
)

install(DIRECTORY include/ DESTINATION include)

install(EXPORT MukhosthoDbTargets
    FILE MukhosthoDbTargets.cmake
    NAMESPACE MukhosthoDb::
    DESTINATION lib/cmake/MukhosthoDb
)

# Add tests (optional)
enable_testing()
add_subdirectory(tests)

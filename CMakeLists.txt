set(ALOH_NETWORKING_DIR "${CMAKE_CURRENT_SOURCE_DIR}")
set(ALOH_NETWORKING_LIB_NAME "libnetworking")
set(ALOH_NETWORKING_ENTRY "./cmd/api/main.go")

find_program(GO_EXECUTABLE go REQUIRED)
find_program(CLANG_C clang REQUIRED)
find_program(CLANG_CXX clang++ REQUIRED)

# Track Go source files so CMake knows when to rebuild
file(GLOB_RECURSE GO_SRCS "${ALOH_NETWORKING_DIR}/*.go")

set(ALOH_NETWORKING_LIB   "${CMAKE_CURRENT_BINARY_DIR}/${ALOH_NETWORKING_LIB_NAME}.a")
set(ALOH_NETWORKING_HEADER     "${CMAKE_CURRENT_BINARY_DIR}/${ALOH_NETWORKING_LIB_NAME}.h")

if(WIN32)
    set(GO_TARGET_OS "windows")
    set(CLANG_TARGET_FLAG "--target=x86_64-w64-mingw32")
else()
    set(GO_TARGET_OS "linux")
    set(CLANG_TARGET_FLAG "--target=x86_64-pc-linux-gnu")
endif()

add_custom_command(
    OUTPUT "${ALOH_NETWORKING_LIB}" "${ALOH_NETWORKING_HEADER}"
    DEPENDS ${GO_SRCS}
    WORKING_DIRECTORY "${ALOH_NETWORKING_DIR}"
    COMMENT "[aloh-networking] Building static library for ${GO_TARGET_OS}..."

    COMMAND ${CMAKE_COMMAND} -E env
        "CC=clang"
        "CXX=clang++"
        "CGO_ENABLED=1"
        "GOOS=${GO_TARGET_OS}"
        "GOARCH=amd64"

    "${GO_EXECUTABLE}" build
            -buildmode=c-archive
            -o "${ALOH_NETWORKING_LIB}"
            -ldflags "-extldflags '-static ${CLANG_TARGET_FLAG}'"

            "${ALOH_NETWORKING_ENTRY}"
)

add_custom_target(build_aloh_networking_target
    ALL
    DEPENDS "${ALOH_NETWORKING_LIB}" "${ALOH_NETWORKING_HEADER}"
    COMMENT "Triggering Go build..."
)

add_library(aloh-networking STATIC IMPORTED GLOBAL)
add_library(aloh::networking ALIAS aloh-networking)

set_target_properties(aloh-networking PROPERTIES
    IMPORTED_LOCATION "${ALOH_NETWORKING_LIB}"
    INTERFACE_INCLUDE_DIRECTORIES "${CMAKE_CURRENT_BINARY_DIR}"
)

add_dependencies(aloh-networking build_aloh_networking_target)
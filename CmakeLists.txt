# ---- aloh-networking: Go shared library built from source ----

set(ALOH_NETWORKING_DIR "${CMAKE_CURRENT_SOURCE_DIR}")
set(GO_LIB_NAME "libnetworking")
set(GO_ENTRY "./cmd/api/main.go")

# --- Find Go ---
find_program(GO_EXECUTABLE go REQUIRED)

# Track Go source files so CMake knows when to rebuild
file(GLOB_RECURSE GO_SRCS "${ALOH_NETWORKING_DIR}/*.go")

# =============================================================
#  WINDOWS
# =============================================================
if(WIN32)
    set(GO_SHARED_LIB "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.dll")
    set(GO_IMPLIB_A   "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.dll.a")
    set(GO_HEADER     "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.h")

    # CGO on Windows requires MinGW GCC regardless of the C++ compiler
    add_custom_command(
        OUTPUT "${GO_SHARED_LIB}" "${GO_IMPLIB_A}" "${GO_HEADER}"
        DEPENDS ${GO_SRCS}
        WORKING_DIRECTORY "${ALOH_NETWORKING_DIR}"
        COMMENT "[aloh-networking] Building Go shared library (Windows)..."
        COMMAND ${CMAKE_COMMAND} -E env
            "CC=x86_64-w64-mingw32-gcc"
            "CGO_ENABLED=1"
            "GOOS=windows"
            "GOARCH=amd64"
            "${GO_EXECUTABLE}" build
                -buildmode=c-shared
                -o "${GO_SHARED_LIB}"
                -ldflags "-extldflags '-static'"
                "${GO_ENTRY}"
    )

    # MSVC's link.exe cannot read MinGW .dll.a files.
    # Generate a .lib from the checked-in .def file.
    if(MSVC)
        set(ALOH_NETWORKING_IMPLIB "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.lib")

        find_program(LIB_TOOL NAMES llvm-lib lib REQUIRED)

        add_custom_command(
            OUTPUT "${ALOH_NETWORKING_IMPLIB}"
            DEPENDS "${GO_SHARED_LIB}"
            COMMAND "${LIB_TOOL}"
                "/def:${ALOH_NETWORKING_DIR}/${GO_LIB_NAME}.def"
                "/out:${ALOH_NETWORKING_IMPLIB}"
                /machine:x64
            COMMENT "[aloh-networking] Generating MSVC import library (.lib) from .def"
        )

        # Explicit target so src/CmakeLists.txt can depend on it
        add_custom_target(aloh-networking-implib ALL
            DEPENDS "${ALOH_NETWORKING_IMPLIB}"
        )
    else()
        # MinGW / Clang (lld-link) can read .dll.a directly
        set(ALOH_NETWORKING_IMPLIB "${GO_IMPLIB_A}")
    endif()

# =============================================================
#  LINUX / UNIX
# =============================================================
else()
    set(GO_SHARED_LIB "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.so")
    set(GO_HEADER     "${CMAKE_CURRENT_BINARY_DIR}/${GO_LIB_NAME}.h")
    set(ALOH_NETWORKING_IMPLIB "")

    add_custom_command(
        OUTPUT "${GO_SHARED_LIB}" "${GO_HEADER}"
        DEPENDS ${GO_SRCS}
        WORKING_DIRECTORY "${ALOH_NETWORKING_DIR}"
        COMMENT "[aloh-networking] Building Go shared library (Linux)..."
        COMMAND ${CMAKE_COMMAND} -E env
            "CGO_ENABLED=1"
            "${GO_EXECUTABLE}" build
                -buildmode=c-shared
                -o "${GO_SHARED_LIB}"
                "${GO_ENTRY}"
    )
endif()

# --- Trigger target ---
add_custom_target(build_aloh_networking ALL DEPENDS "${GO_SHARED_LIB}")

# --- IMPORTED library target ---
add_library(aloh-networking SHARED IMPORTED GLOBAL)
add_dependencies(aloh-networking build_aloh_networking)

# Header from the source tree (the committed libnetworking.h)
set_target_properties(aloh-networking PROPERTIES
    INTERFACE_INCLUDE_DIRECTORIES "${ALOH_NETWORKING_DIR}"
)

if(WIN32)
    set_target_properties(aloh-networking PROPERTIES
        IMPORTED_LOCATION "${GO_SHARED_LIB}"
        IMPORTED_IMPLIB   "${ALOH_NETWORKING_IMPLIB}"
    )
else()
    set_target_properties(aloh-networking PROPERTIES
        IMPORTED_LOCATION "${GO_SHARED_LIB}"
    )
endif()
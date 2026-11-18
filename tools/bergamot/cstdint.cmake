# GCC 13 needs <cstdint> in vendored faiss, but marian overwrites
# CMAKE_CXX_FLAGS, so a top-level include is the only reliable injection point.
# Restrict it to C++ so the C sources are untouched.
add_compile_options("$<$<COMPILE_LANGUAGE:CXX>:SHELL:-include cstdint>")

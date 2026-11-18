# Reproducible native Bergamot build environment.
#
# Pinned to the Ubuntu 24.04 image used to verify the v0.4.5 build. The image
# carries only the C++ toolchain; Go is provided by the host when the cgo bridge
# is compiled.
FROM ubuntu:24.04@sha256:73b999393d07f2bf65a40373778a44dc9a6d54ad7718d29247d8ff9413c4d6a8

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        cmake \
        git \
        libboost-filesystem-dev \
        libboost-program-options-dev \
        libboost-system-dev \
        libboost-thread-dev \
        libopenblas-dev \
        libpcre2-dev \
        libssl-dev \
        pkg-config \
        zlib1g-dev \
    && rm -rf /var/lib/apt/lists/*

# Native Bergamot needs a C++17 compiler. GCC 13 in 24.04 builds the pinned
# v0.4.5 sources without source patches.
WORKDIR /work

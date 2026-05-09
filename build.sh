#!/bin/bash
# Tesla Streamer - High-performance screen streaming for Tesla browsers
# Copyright (C) 2026 Jaroslav Reznik
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

set -e

BUILD_BACKEND=false
BUILD_GUI=false
BUILD_FLATPAK=false
BUILD_DECKY=false

show_help() {
    echo "Usage: ./build.sh [options]"
    echo ""
    echo "Options:"
    echo "  --backend    Build the Go backend binary"
    echo "  --gui        Build the C++ Qt GUI application"
    echo "  --flatpak    Build the Flatpak package"
    echo "  --decky      Build the Decky Loader plugin"
    echo "  --all        Build all of the above"
    echo "  --help       Show this help message"
    echo ""
}

if [[ $# -eq 0 ]]; then
    show_help
    exit 0
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --backend)
            BUILD_BACKEND=true
            shift
            ;;
        --gui)
            BUILD_GUI=true
            shift
            ;;
        --flatpak)
            BUILD_FLATPAK=true
            shift
            ;;
        --decky)
            BUILD_DECKY=true
            shift
            ;;
        --all)
            BUILD_BACKEND=true
            BUILD_GUI=true
            BUILD_FLATPAK=true
            BUILD_DECKY=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

if [ "$BUILD_BACKEND" = true ]; then
    echo "--- Building Go Backend ---"
    export CGO_ENABLED=1
    go build -ldflags "-linkmode external -extldflags '-static-libgcc -static-libstdc++'" -o tesla-streamer .
    echo "Go backend built: ./tesla-streamer"
fi

if [ "$BUILD_GUI" = true ]; then
    echo "--- Building C++ Qt GUI ---"
    mkdir -p gui/build
    cd gui/build
    cmake ..
    make -j$(nproc)
    cd ../..
    echo "C++ GUI built: gui/build/tesla-streamer-gui"
fi

if [ "$BUILD_FLATPAK" = true ]; then
    echo "--- Building Flatpak Package ---"
    if ! command -v flatpak-builder &> /dev/null; then
        echo "ERROR: flatpak-builder not found. Please install it to build the Flatpak."
        exit 1
    fi
    flatpak-builder --force-clean build-dir io.github.jreznik.TeslaStreamer.yaml
    echo "Flatpak build complete in build-dir/"
fi

if [ "$BUILD_DECKY" = true ]; then
    echo "--- Building Decky Loader Plugin ---"
    if ! command -v pnpm &> /dev/null; then
        echo "ERROR: pnpm not found. Please install pnpm to build the Decky plugin."
        exit 1
    fi
    cd decky-plugin
    pnpm install
    pnpm run build
    cd ..
    echo "Decky plugin build complete in decky-plugin/dist/"
fi

echo ""
echo "Done!"

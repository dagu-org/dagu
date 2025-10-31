#!/bin/bash
# build-windows.sh - Complete cross-compilation script

# Build UI assets
echo "Building UI assets..."
cd ui && pnpm install --frozen-lockfile && pnpm build && cd ..
cp -r ui/dist/* internal/service/frontend/assets/

# Build for Windows platforms
echo "Cross-compiling for Windows x86-64..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -o dist/dagu-windows-amd64.exe ./cmd

echo "Cross-compiling for Windows ARM64..."
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags)" \
    -o dist/dagu-windows-arm64.exe ./cmd

echo "Build complete! Binaries in ./dist/"

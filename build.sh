#!/bin/bash
# Build multi-platform binaries

set -e

cd "$(dirname "$0")/src"

# Create bin directory
mkdir -p ../bin

echo "Downloading dependencies..."
go mod tidy

echo "Building..."

# macOS ARM64 (Apple Silicon)
echo "  -> darwin-arm64"
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../bin/file-share-darwin-arm64 .

# macOS AMD64 (Intel)
echo "  -> darwin-amd64"
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/file-share-darwin-amd64 .

# Linux AMD64
echo "  -> linux-amd64"
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/file-share-linux-amd64 .

# Linux ARM64
echo "  -> linux-arm64"
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../bin/file-share-linux-arm64 .

# Windows AMD64
echo "  -> windows-amd64"
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/file-share-windows-amd64.exe .

echo ""
echo "✅ Build complete! Binaries in bin/ directory:"
ls -lh ../bin/

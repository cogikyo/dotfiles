#!/bin/bash
# Install script for Claude Code statusline
# Works on macOS and Linux

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_BIN="$SCRIPT_DIR/../statusline-bin"

# Check for Go
if ! command -v go &>/dev/null; then
    echo "Error: Go is not installed"
    echo ""
    echo "Install Go:"
    echo "  macOS:  brew install go"
    echo "  Linux:  sudo pacman -S go  (or apt install golang-go)"
    echo "  Other:  https://go.dev/dl/"
    exit 1
fi

# Check Go version (need 1.22+ for range over int)
GO_VERSION=$(go version | sed 's/.*go\([0-9]*\.[0-9]*\).*/\1/')
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

if [[ "$GO_MAJOR" -lt 1 ]] || [[ "$GO_MAJOR" -eq 1 && "$GO_MINOR" -lt 22 ]]; then
    echo "Error: Go 1.22+ required (found $GO_VERSION)"
    exit 1
fi

echo "Building statusline binary..."
cd "$SCRIPT_DIR"
go build -ldflags="-s -w" -o "$OUTPUT_BIN" .

echo "Built: $OUTPUT_BIN"
echo ""
echo "Add to Claude Code settings.json:"
echo "  \"statusLine\": {"
echo "    \"type\": \"command\","
echo "    \"command\": \"$OUTPUT_BIN\""
echo "  }"

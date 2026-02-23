#!/usr/bin/env bash
# Build and install Vagari font from Iosevka

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOTFILES="$(cd "$SCRIPT_DIR/../.." && pwd)"
IOSEVKA_DIR="$HOME/downloads/Iosevka"
FONT_DIR="$HOME/.local/share/fonts"
FONT_NAME="Vagari"
TAR_FILE="$DOTFILES/etc/fonts.tar.gz"

# ---------------------------------------------------------------------------
#  --fonts: rebuild tarball from installed fonts and exit
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--fonts" ]]; then
    if [[ ! -d "$FONT_DIR" ]] || [[ -z "$(ls -A "$FONT_DIR" 2>/dev/null)" ]]; then
        echo "Error: $FONT_DIR is empty — nothing to archive"
        exit 1
    fi
    echo "Rebuilding $TAR_FILE from $FONT_DIR..."
    tar -czf "$TAR_FILE" -C "$HOME/.local/share" fonts
    echo ""
    echo "Archived $(find "$FONT_DIR" -type f | wc -l) fonts ($(du -h "$TAR_FILE" | cut -f1)):"
    find "$FONT_DIR" -type f -printf '  %f\n' | sort
    exit 0
fi

# ---------------------------------------------------------------------------
#  Build Vagari from Iosevka source
# ---------------------------------------------------------------------------
MAX_JOBS="${1:-8}"  # Limit parallel jobs (each uses ~1GB RAM), override with: ./build.sh N

# Clone Iosevka if needed
if [[ ! -d "$IOSEVKA_DIR" ]]; then
    echo "Cloning Iosevka..."
    git clone --depth 1 https://github.com/be5invis/Iosevka.git "$IOSEVKA_DIR"
fi

# Copy build plan
echo "Copying build plan..."
cp "$SCRIPT_DIR/private-build-plans.toml" "$IOSEVKA_DIR/"

# Install dependencies if needed
if [[ ! -d "$IOSEVKA_DIR/node_modules" ]]; then
    echo "Installing dependencies (first time only)..."
    cd "$IOSEVKA_DIR"
    npm install
fi

# Build font (unhinted TTF + WOFF2)
echo "Building $FONT_NAME with $MAX_JOBS parallel jobs..."
cd "$IOSEVKA_DIR"
npm run build -- ttf-unhinted::"$FONT_NAME" webfont-unhinted::"$FONT_NAME" --jCmd="$MAX_JOBS"

# Remove old fonts
echo "Removing old fonts..."
rm -f "$FONT_DIR"/IosevkaVagari*.ttf
rm -f "$FONT_DIR"/Vagari*.ttf
rm -f "$FONT_DIR"/IosevkaVagari*.woff2
rm -f "$FONT_DIR"/Vagari*.woff2

# Install new fonts
echo "Installing new fonts..."
mkdir -p "$FONT_DIR"
cp "$IOSEVKA_DIR/dist/$FONT_NAME/TTF-Unhinted/"*.ttf "$FONT_DIR/"
cp "$IOSEVKA_DIR/dist/$FONT_NAME/WOFF2-Unhinted/"*.woff2 "$FONT_DIR/"

# Refresh font cache
echo "Refreshing font cache..."
fc-cache -f

echo ""
echo "Done! Installed fonts:"
fc-list | grep -i vagari | head -5

echo ""
echo "To update the dotfiles tarball with these fonts, run:"
echo "  $0 --fonts"

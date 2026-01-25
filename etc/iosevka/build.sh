#!/usr/bin/env bash
# Build and install Vagari font from Iosevka

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IOSEVKA_DIR="$HOME/downloads/Iosevka"
FONT_DIR="$HOME/.local/share/fonts"
FONT_NAME="Vagari"
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

# Build font
echo "Building $FONT_NAME with $MAX_JOBS parallel jobs..."
cd "$IOSEVKA_DIR"
npm run build -- ttf::"$FONT_NAME" --jCmd="$MAX_JOBS"

# Remove old fonts
echo "Removing old fonts..."
rm -f "$FONT_DIR"/IosevkaVagari*.ttf
rm -f "$FONT_DIR"/Vagari*.ttf
rm -f "$FONT_DIR"/IosevkaVagari*.woff2
rm -f "$FONT_DIR"/Vagari*.woff2

# Install new fonts
echo "Installing new fonts..."
mkdir -p "$FONT_DIR"
cp "$IOSEVKA_DIR/dist/$FONT_NAME/TTF/"*.ttf "$FONT_DIR/"

# Refresh font cache
echo "Refreshing font cache..."
fc-cache -fv

echo ""
echo "Done! Installed fonts:"
fc-list | grep -i vagari | head -5

echo ""
echo "Restart Kitty (ctrl+shift+f5) and use these settings:"
echo ""
echo "  font_family      $FONT_NAME"
echo "  italic_font      $FONT_NAME Italic"
echo "  bold_font        $FONT_NAME Bold"
echo "  bold_italic_font $FONT_NAME Bold Italic"

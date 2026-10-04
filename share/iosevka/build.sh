#!/usr/bin/env bash
# Build hinted Vagari from a pinned Iosevka release into share/fonts.

set -euo pipefail
shopt -s nullglob

VERSION="v34.1.0"
TTFAUTOHINT="ttfautohint-py==0.6.1"
JOBS="${1:-4}"

DOTFILES="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FONTS="$DOTFILES/share/fonts"
SRC="${XDG_CACHE_HOME:-$HOME/.cache}/iosevka/$VERSION"
DIST="$SRC/dist/Vagari/TTF"

if [[ ! -d "$SRC" ]]; then
    git clone --quiet --depth 1 --branch "$VERSION" https://github.com/be5invis/Iosevka.git "$SRC"
fi
cp "$DOTFILES/share/iosevka/private-build-plans.toml" "$SRC/"
if [[ ! -d "$SRC/node_modules" ]]; then
    (cd "$SRC" && npm ci --no-audit --no-fund)
fi

mkdir -p "$SRC/.bin"
cat > "$SRC/.bin/ttfautohint" <<EOF
#!/usr/bin/env bash
exec uv run --quiet --no-project --with $TTFAUTOHINT python -m ttfautohint "\$@"
EOF
chmod +x "$SRC/.bin/ttfautohint"

(cd "$SRC" && TTFAUTOHINT_PATH="$SRC/.bin/ttfautohint" npm run build -- ttf::Vagari --jCmd="$JOBS")

mkdir -p "$FONTS"
for font in "$FONTS"/Vagari-*.ttf; do
    [[ -e "$DIST/${font##*/}" ]] || trash -- "$font"
done
cp "$DIST"/*.ttf "$FONTS/"
fc-cache
fc-list Vagari family style file | sort

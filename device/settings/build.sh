#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
SOURCE="$ROOT/device/settings"
OUTPUT=${1:-"$ROOT/dist/paper-agent-settings"}
TARGET=${PAPER_AGENT_SETTINGS_TARGET:-aarch64-unknown-linux-gnu}
RCC=${RCC:-rcc}

command -v "$RCC" >/dev/null
command -v cargo >/dev/null

rm -rf "$OUTPUT"
mkdir -p "$OUTPUT/backend"
"$RCC" --binary -o "$OUTPUT/resources.rcc" "$SOURCE/application.qrc"
cargo build \
  --manifest-path "$SOURCE/backend/Cargo.toml" \
  --target "$TARGET" \
  --release \
  --locked
cp "$SOURCE/backend/target/$TARGET/release/paper-agent-settings-backend" \
  "$OUTPUT/backend/entry"
cp "$SOURCE/manifest.json" "$OUTPUT/manifest.json"

if command -v rsvg-convert >/dev/null; then
  rsvg-convert -w 256 -h 256 "$SOURCE/icon.svg" -o "$OUTPUT/icon.png"
elif command -v magick >/dev/null; then
  magick -background none "$SOURCE/icon.svg" -resize 256x256 "$OUTPUT/icon.png"
elif command -v convert >/dev/null; then
  convert -background none "$SOURCE/icon.svg" -resize 256x256 "$OUTPUT/icon.png"
elif command -v sips >/dev/null; then
  sips -s format png "$SOURCE/icon.svg" --out "$OUTPUT/icon.png" >/dev/null
else
  echo "rsvg-convert, ImageMagick, or sips is required to build icon.png" >&2
  exit 127
fi

chmod 0755 "$OUTPUT/backend/entry"
chmod 0644 "$OUTPUT/manifest.json" "$OUTPUT/resources.rcc" "$OUTPUT/icon.png"
echo "settings_app=$OUTPUT"

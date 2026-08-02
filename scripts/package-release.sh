#!/usr/bin/env bash
set -euo pipefail

# Build one deterministic, device-ready Paper Agent release bundle and its
# public manifest. ARM64 binaries must be built before running this script.

if [ "$#" -lt 5 ] || [ "$#" -gt 6 ]; then
  echo "usage: $0 VERSION NATIVE_BINARY IMAGE_PLUGIN SETTINGS_BUNDLE PI_RUNTIME_BUNDLE [OUTPUT_DIR]" >&2
  exit 2
fi

VERSION=$1
NATIVE=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
IMAGE=$(cd "$(dirname "$3")" && pwd)/$(basename "$3")
SETTINGS=$(cd "$4" && pwd)
PI_RUNTIME=$(cd "$(dirname "$5")" && pwd)/$(basename "$5")
ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUTPUT=${6:-"$ROOT/dist/release"}
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-release.XXXXXX")
TAR=${TAR:-tar}

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

case "$VERSION" in
  ''|*[!0-9A-Za-z._+-]*) echo "invalid release version: $VERSION" >&2; exit 1 ;;
esac
"$TAR" --version 2>/dev/null | grep -q 'GNU tar' || {
  echo "GNU tar is required for deterministic release packaging" >&2
  exit 1
}
test -x "$NATIVE"
test -x "$IMAGE"
file "$NATIVE" | grep -Eq 'ARM aarch64|ARM64'
file "$IMAGE" | grep -Eq 'ARM aarch64|ARM64'
test -f "$SETTINGS/manifest.json"
test -f "$SETTINGS/resources.rcc"
test -f "$SETTINGS/icon.png"
test -x "$SETTINGS/backend/entry"
file "$SETTINGS/backend/entry" | grep -Eq 'ARM aarch64|ARM64'
test -s "$PI_RUNTIME"

BUNDLE="$STAGE/bundle"
PAYLOAD="$BUNDLE/payload"
LICENSES="$BUNDLE/licenses"
mkdir -p "$PAYLOAD/runtime/skills" "$PAYLOAD/assets/icons" "$LICENSES/fonts"
printf '%s\n' "$VERSION" >"$BUNDLE/VERSION"
cp "$ROOT/LICENSE" "$LICENSES/PAPER-AGENT-MIT.txt"
cp "$ROOT/THIRD_PARTY_NOTICES.md" "$LICENSES/"
cp "$ROOT/third_party/"*.txt "$LICENSES/"
cp "$ROOT/device/native/assets/fonts/"*-OFL.txt "$LICENSES/fonts/"
cp "$ROOT/device/native/assets/fonts/"*LICENSE.txt "$LICENSES/fonts/"
cp "$ROOT/device/native/assets/opencc/OPENCC-LICENSE.txt" "$LICENSES/"
cat >"$LICENSES/SOURCE.txt" <<EOF
Paper Agent source for release $VERSION:
https://github.com/bonjouryentinglai/PaperAgent

The matching Git tag and GitHub-generated source archive are the corresponding
source for Paper Agent's GPL components. External dependencies retain the
versions and source links recorded in THIRD_PARTY_NOTICES.md.
EOF
cp "$ROOT/device/release/install.sh" "$BUNDLE/install.sh"
cp "$NATIVE" "$PAYLOAD/paper-agent-native"
cp "$IMAGE" "$PAYLOAD/paper-agent-image.so"
for file in "$ROOT/device/runtime/"*.mjs "$ROOT/device/runtime/"*.ts "$ROOT/device/runtime/"*.sh; do
  cp "$file" "$PAYLOAD/runtime/"
done
cp -R "$ROOT/device/runtime/skills/"* "$PAYLOAD/runtime/skills/"
cp "$ROOT/docs/assets/icons/paper-agent-ai.svg" "$PAYLOAD/assets/icons/"
cp "$ROOT/docs/assets/icons/paper-agent-beautify.svg" "$PAYLOAD/assets/icons/"
cp "$ROOT/device/systemd/paper-agent-native-oracle.service" "$PAYLOAD/"
cp "$ROOT/device/systemd/paper-agent-xovi-post-start.sh" "$PAYLOAD/"
cp "$ROOT/device/qmd/paper-agent-selection.qmd" "$PAYLOAD/"
cp "$ROOT/config/paper-agent.env.example" "$PAYLOAD/"
cp -R "$SETTINGS" "$PAYLOAD/settings-app"

chmod 0755 "$BUNDLE/install.sh" "$PAYLOAD/paper-agent-native" \
  "$PAYLOAD/paper-agent-image.so" "$PAYLOAD/runtime/"*.sh \
  "$PAYLOAD/paper-agent-xovi-post-start.sh" \
  "$PAYLOAD/settings-app/backend/entry"
find "$BUNDLE" -type f ! -perm -0100 -exec chmod 0644 {} +
find "$BUNDLE" -type d -exec chmod 0755 {} +

rm -rf "$OUTPUT"
mkdir -p "$OUTPUT"
cp "$PI_RUNTIME" "$OUTPUT/paper-agent-pi-runtime.tar.gz"
"$TAR" --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 \
  --numeric-owner -cf - -C "$BUNDLE" . \
  | gzip -n -9 >"$OUTPUT/paper-agent-release.tar.gz"

SHA=$(shasum -a 256 "$OUTPUT/paper-agent-release.tar.gz" | awk '{print $1}')
BYTES=$(wc -c <"$OUTPUT/paper-agent-release.tar.gz" | tr -d ' ')
RUNTIME_SHA=$(shasum -a 256 "$OUTPUT/paper-agent-pi-runtime.tar.gz" | awk '{print $1}')
RUNTIME_BYTES=$(wc -c <"$OUTPUT/paper-agent-pi-runtime.tar.gz" | tr -d ' ')
cat >"$OUTPUT/paper-agent-manifest.json" <<EOF
{
  "schema": 2,
  "version": "$VERSION",
  "supportedModels": ["chiappa"],
  "minimumFreeSpaceKB": 262144,
  "bundle": {
    "url": "paper-agent-release.tar.gz",
    "sha256": "$SHA",
    "bytes": $BYTES
  },
  "runtime": {
    "url": "paper-agent-pi-runtime.tar.gz",
    "sha256": "$RUNTIME_SHA",
    "bytes": $RUNTIME_BYTES
  }
}
EOF
printf '%s  %s\n%s  %s\n' \
  "$SHA" paper-agent-release.tar.gz \
  "$RUNTIME_SHA" paper-agent-pi-runtime.tar.gz \
  >"$OUTPUT/SHA256SUMS"
echo "release_bundle=$OUTPUT/paper-agent-release.tar.gz"
echo "release_manifest=$OUTPUT/paper-agent-manifest.json"

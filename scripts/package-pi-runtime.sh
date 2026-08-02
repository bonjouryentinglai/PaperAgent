#!/usr/bin/env bash
set -euo pipefail

# Build the exact ARM64 Node + Pi runtime used by the desktop installer. Run
# this on an ARM64 Linux host so optional native npm packages match the Move.

if [ "$#" -gt 1 ]; then
  echo "usage: $0 [OUTPUT_DIR]" >&2
  exit 2
fi

ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUTPUT=${1:-"$ROOT/dist/runtime"}
NODE_VERSION=22.22.3
NODE_ARCHIVE="node-v${NODE_VERSION}-linux-arm64.tar.gz"
NODE_URL="https://nodejs.org/download/release/v${NODE_VERSION}/${NODE_ARCHIVE}"
NODE_SHA256=cc8bc82b2dd0b595c3b95a4c3c9c8c350907cff011afbdee3d1379e812e1e3e3
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-pi-runtime.XXXXXX")
TAR=${TAR:-tar}

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

if [ "$(uname -s)" != Linux ]; then
  echo "package-pi-runtime.sh must run on ARM64 Linux" >&2
  exit 1
fi
case "$(uname -m)" in
  aarch64|arm64) ;;
  *) echo "package-pi-runtime.sh must run on ARM64 Linux" >&2; exit 1 ;;
esac
"$TAR" --version 2>/dev/null | grep -q 'GNU tar' || {
  echo "GNU tar is required for deterministic runtime packaging" >&2
  exit 1
}

curl --fail --location --silent --show-error "$NODE_URL" -o "$STAGE/$NODE_ARCHIVE"
printf '%s  %s\n' "$NODE_SHA256" "$STAGE/$NODE_ARCHIVE" | sha256sum --check --status

NODE_DIR="$STAGE/node-v${NODE_VERSION}"
RUNTIME_APP="$NODE_DIR/lib/paper-agent-runtime"
mkdir -p "$NODE_DIR" "$RUNTIME_APP" "$STAGE/home" "$STAGE/npm-cache"
"$TAR" -xzf "$STAGE/$NODE_ARCHIVE" -C "$NODE_DIR" --strip-components=1

cp "$ROOT/installer/runtime/package.json" "$RUNTIME_APP/package.json"
cp "$ROOT/installer/runtime/package-lock.json" "$RUNTIME_APP/package-lock.json"
HOME="$STAGE/home" npm_config_cache="$STAGE/npm-cache" \
  "$NODE_DIR/bin/npm" ci --prefix "$RUNTIME_APP" \
  --omit=dev --no-audit --no-fund
ln -s ../lib/paper-agent-runtime/node_modules/@earendil-works/pi-coding-agent/dist/cli.js \
  "$NODE_DIR/bin/pi"
ln -s ../lib/paper-agent-runtime/node_modules/@earendil-works/pi-ai/dist/cli.js \
  "$NODE_DIR/bin/pi-ai"

test -x "$NODE_DIR/bin/node"
test -x "$NODE_DIR/bin/pi"
test -x "$NODE_DIR/bin/pi-ai"
"$NODE_DIR/bin/node" --version | grep -qx "v${NODE_VERSION}"
HOME="$STAGE/home" PATH="$NODE_DIR/bin:$PATH" "$NODE_DIR/bin/pi" --version >/dev/null

rm -rf "$OUTPUT"
mkdir -p "$OUTPUT"
"$TAR" --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 \
  --numeric-owner -cf - -C "$STAGE" "node-v${NODE_VERSION}" \
  | gzip -n -9 >"$OUTPUT/paper-agent-pi-runtime.tar.gz"

sha256sum "$OUTPUT/paper-agent-pi-runtime.tar.gz" \
  >"$OUTPUT/paper-agent-pi-runtime.tar.gz.sha256"
echo "runtime_bundle=$OUTPUT/paper-agent-pi-runtime.tar.gz"

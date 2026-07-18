#!/usr/bin/env bash
set -euo pipefail

# Install a pinned ARM64 Node + Pi runtime below /home/root. This script does
# not log in, install Paper Agent, or modify Xochitl.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
NODE_VERSION=${PAPER_AGENT_NODE_VERSION:-22.22.3}
PI_VERSION=${PAPER_AGENT_PI_VERSION:-0.80.7}
DEVICE="${DEVICE_USER}@${HOST}"
ARCHIVE="node-v${NODE_VERSION}-linux-arm64.tar.gz"
BASE_URL="https://nodejs.org/download/release/v${NODE_VERSION}"
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-runtime.XXXXXX")

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

curl --fail --silent --show-error --location "$BASE_URL/$ARCHIVE" --output "$STAGE/$ARCHIVE"
curl --fail --silent --show-error --location "$BASE_URL/SHASUMS256.txt" --output "$STAGE/SHASUMS256.txt"
(
  cd "$STAGE"
  grep "  ${ARCHIVE}\$" SHASUMS256.txt | shasum -a 256 -c -
)

scp -O -o BatchMode=yes "$STAGE/$ARCHIVE" "$DEVICE:/home/root/$ARCHIVE"
ssh -o BatchMode=yes "$DEVICE" "NODE_VERSION='$NODE_VERSION' ARCHIVE='$ARCHIVE' PI_VERSION='$PI_VERSION' sh -s" <<'DEVICE_SCRIPT'
set -eu
umask 022

RUNTIME=/home/root/paper-agent/runtime
NODE_DIR="$RUNTIME/node-v$NODE_VERSION"
STAGING="$NODE_DIR.staging"

mkdir -p "$RUNTIME"
if [ ! -x "$NODE_DIR/bin/node" ]; then
  rm -rf "$STAGING"
  mkdir -p "$STAGING"
  tar -xzf "/home/root/$ARCHIVE" -C "$STAGING" --strip-components=1
  mv "$STAGING" "$NODE_DIR"
fi
rm -f "/home/root/$ARCHIVE"

if [ -e /home/root/node ] && [ ! -L /home/root/node ]; then
  echo "/home/root/node exists and is not a symlink; refusing to replace it" >&2
  exit 1
fi
ln -sfn "$NODE_DIR" /home/root/node

HOME=/home/root PATH="/home/root/node/bin:$PATH" \
  /home/root/node/bin/npm install --global --prefix /home/root/node \
  --omit=dev --no-audit --no-fund \
  "@earendil-works/pi-coding-agent@$PI_VERSION" \
  "@earendil-works/pi-ai@$PI_VERSION"

/home/root/node/bin/node --version
HOME=/home/root PATH="/home/root/node/bin:$PATH" /home/root/node/bin/pi --version
test -x /home/root/node/bin/pi-ai
DEVICE_SCRIPT

echo "Runtime installed. Run scripts/login.sh next."

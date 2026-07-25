#!/usr/bin/env bash
set -euo pipefail

# Install one prebuilt Paper Agent Settings AppLoad bundle. This does not
# restart Xochitl; AppLoad's Reload action discovers the new app.

if [ "$#" -ne 1 ]; then
  echo "usage: $0 /path/to/paper-agent-settings-bundle" >&2
  exit 2
fi

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"
BUNDLE=$(cd "$1" && pwd)
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-settings-install.XXXXXX")

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

test -f "$BUNDLE/manifest.json"
test -f "$BUNDLE/resources.rcc"
test -f "$BUNDLE/icon.png"
test -x "$BUNDLE/backend/entry"
file "$BUNDLE/backend/entry" | grep -Eq 'ARM aarch64|ARM64' || {
  echo "Settings backend is not ARM64: $BUNDLE/backend/entry" >&2
  exit 1
}
grep -q '"id"[[:space:]]*:[[:space:]]*"paper-agent-settings"' "$BUNDLE/manifest.json"
tar -czf "$STAGE/paper-agent-settings.tar.gz" -C "$BUNDLE" .
scp -O -o BatchMode=yes "$STAGE/paper-agent-settings.tar.gz" \
  "$DEVICE:/tmp/paper-agent-settings.tar.gz"

ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -eu
APPLOAD=/home/root/xovi/exthome/appload
TARGET="$APPLOAD/paper-agent-settings"
BASE=/home/root/paper-agent
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$BASE/backups/settings-install-$STAMP"
INCOMING="$APPLOAD/.paper-agent-settings-incoming.$$"

cleanup() {
  rm -rf "$INCOMING"
  rm -f /tmp/paper-agent-settings.tar.gz
}
trap cleanup EXIT

test -f /home/root/xovi/extensions.d/appload.so
mkdir -p "$APPLOAD" "$INCOMING" "$BACKUP"
tar -xzf /tmp/paper-agent-settings.tar.gz -C "$INCOMING"
test -f "$INCOMING/manifest.json"
test -f "$INCOMING/resources.rcc"
test -f "$INCOMING/icon.png"
test -x "$INCOMING/backend/entry"
grep -q '"id"[[:space:]]*:[[:space:]]*"paper-agent-settings"' "$INCOMING/manifest.json"

if [ -d "$TARGET" ]; then cp -a "$TARGET" "$BACKUP/paper-agent-settings"; fi
chmod 0755 "$INCOMING/backend" "$INCOMING/backend/entry"
chmod 0644 "$INCOMING/manifest.json" "$INCOMING/resources.rcc" "$INCOMING/icon.png"
rm -rf "$TARGET.previous"
if [ -d "$TARGET" ]; then mv "$TARGET" "$TARGET.previous"; fi
if mv "$INCOMING" "$TARGET"; then
  rm -rf "$TARGET.previous"
else
  rm -rf "$TARGET"
  if [ -d "$TARGET.previous" ]; then mv "$TARGET.previous" "$TARGET"; fi
  echo "Paper Agent Settings install rolled back" >&2
  exit 1
fi

echo "paper_agent_settings=installed"
echo "appload_action=reload"
DEVICE_SCRIPT

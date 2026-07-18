#!/usr/bin/env bash
set -euo pipefail

# Install the pinned XOVI extensions required by the QMD. XOVI itself and
# qt-resource-rebuilder must already be installed through remagic/AppLoad.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
CACHE=${PAPER_AGENT_CACHE:-"$ROOT/.cache/xovi"}
RM_SHOT="$CACHE/rm-shot-aarch64.so"
XOVI_ARCHIVE="$CACHE/xovi-aarch64.tar.gz"
FRAMEBUFFER_SPY="$CACHE/framebuffer-spy.so"
COMMAND_EXECUTOR="$CACHE/qt-command-executor.so"

RM_SHOT_URL=https://github.com/rmitchellscott/rm-shot/releases/download/v1.2.0/rm-shot-aarch64.so
RM_SHOT_SHA=1526fb4a0582c26801afc2861eeafd651286ef0f23bbdb372700dd025f66f786
XOVI_URL=https://github.com/asivery/rm-xovi-extensions/releases/download/v19-23052026/xovi-aarch64.tar.gz
XOVI_SHA=32d64d1262ddc984e3235c7d0340a398fe6d5b3efa6a979865f5977b32630d27

mkdir -p "$CACHE"
[ -f "$RM_SHOT" ] || curl --fail --location "$RM_SHOT_URL" --output "$RM_SHOT"
[ -f "$XOVI_ARCHIVE" ] || curl --fail --location "$XOVI_URL" --output "$XOVI_ARCHIVE"
printf '%s  %s\n' "$RM_SHOT_SHA" "$RM_SHOT" | shasum -a 256 -c -
printf '%s  %s\n' "$XOVI_SHA" "$XOVI_ARCHIVE" | shasum -a 256 -c -
tar -xOzf "$XOVI_ARCHIVE" xovi/inactive-extensions/framebuffer-spy.so > "$FRAMEBUFFER_SPY"
tar -xOzf "$XOVI_ARCHIVE" xovi/inactive-extensions/qt-command-executor.so > "$COMMAND_EXECUTOR"
chmod 755 "$FRAMEBUFFER_SPY" "$COMMAND_EXECUTOR" "$RM_SHOT"

scp -O -o BatchMode=yes "$RM_SHOT" "$FRAMEBUFFER_SPY" "$COMMAND_EXECUTOR" "$DEVICE:/tmp/"
ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -eu

EXT=/home/root/xovi/extensions.d
STATE_ROOT=/home/root/paper-agent/backups
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$STATE_ROOT/xovi-deps-$STAMP"
test -d "$EXT"
mkdir -p "$STATE_ROOT" "$BACKUP"

for name in framebuffer-spy.so rm-shot-aarch64.so qt-command-executor.so; do
  if [ -f "$EXT/$name" ]; then
    cp -p "$EXT/$name" "$BACKUP/$name"
    echo present > "$BACKUP/$name.state"
  else
    echo absent > "$BACKUP/$name.state"
  fi
done

cp /tmp/framebuffer-spy.so "$EXT/framebuffer-spy.so"
cp /tmp/rm-shot-aarch64.so "$EXT/rm-shot-aarch64.so"
cp /tmp/qt-command-executor.so "$EXT/qt-command-executor.so"
chmod 755 "$EXT/framebuffer-spy.so" "$EXT/rm-shot-aarch64.so" "$EXT/qt-command-executor.so"
rm -f /tmp/framebuffer-spy.so /tmp/rm-shot-aarch64.so /tmp/qt-command-executor.so
echo "$BACKUP" > /home/root/paper-agent/xovi-deps-last-backup
echo "xovi_dependencies=installed"
DEVICE_SCRIPT

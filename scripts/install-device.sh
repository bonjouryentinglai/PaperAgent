#!/usr/bin/env bash
set -euo pipefail

# Transactionally install one prebuilt ARM64 native binary, the runtime and the
# QMD. Required XOVI dependencies must be installed first.

if [ "$#" -ne 1 ]; then
  echo "usage: $0 /path/to/paper-agent-native" >&2
  exit 2
fi

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
BIN=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-install.XXXXXX")
if [ -n "${PAPER_AGENT_LOCAL_NODE:-}" ]; then
  NODE_CHECK=$PAPER_AGENT_LOCAL_NODE
elif [ -x /usr/local/opt/node@22/bin/node ]; then
  NODE_CHECK=/usr/local/opt/node@22/bin/node
else
  NODE_CHECK=node
fi

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

test -x "$BIN"
file "$BIN" | grep -Eq 'ARM aarch64|ARM64' || {
  echo "native binary is not ARM64: $BIN" >&2
  exit 1
}

"$NODE_CHECK" --check "$ROOT/device/runtime/native-oracle-server.mjs"
"$NODE_CHECK" "$ROOT/device/runtime/native-oracle-server.mjs" --self-test
"$NODE_CHECK" --check "$ROOT/device/runtime/native-oracle-client.mjs"
"$NODE_CHECK" --check "$ROOT/device/runtime/rich-document.mjs"
"$NODE_CHECK" --test "$ROOT/device/runtime/rich-document.test.mjs"
"$NODE_CHECK" --check "$ROOT/device/runtime/image-generate.mjs"
"$NODE_CHECK" --test "$ROOT/device/runtime/image-generate.test.mjs"
sh -n "$ROOT/device/runtime/native-oracle-service.sh"
sh -n "$ROOT/device/runtime/native-selection-prepare.sh"
sh -n "$ROOT/device/runtime/native-selection-write.sh"

mkdir -p "$STAGE/payload/runtime"
cp "$BIN" "$STAGE/payload/paper-agent-native"
cp "$ROOT/device/runtime/"* "$STAGE/payload/runtime/"
cp "$ROOT/device/systemd/paper-agent-native-oracle.service" "$STAGE/payload/"
cp "$ROOT/device/qmd/paper-agent-selection.qmd" "$STAGE/payload/"
cp "$ROOT/config/paper-agent.env.example" "$STAGE/payload/"
tar -czf "$STAGE/paper-agent-payload.tar.gz" -C "$STAGE/payload" .
scp -O -o BatchMode=yes "$STAGE/paper-agent-payload.tar.gz" "$DEVICE:/tmp/paper-agent-payload.tar.gz"

ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -eu

BASE=/home/root/paper-agent
NATIVE="$BASE/native"
QMD_HOME=/home/root/xovi/exthome/qt-resource-rebuilder
QMD_FILE="$QMD_HOME/paperAgentSelection.qmd"
UNIT=/etc/systemd/system/paper-agent-native-oracle.service
CONFIG="$BASE/config.env"
STATE_ROOT="$BASE/backups"
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$STATE_ROOT/install-$STAMP"
INCOMING="$BASE/.install-incoming.$$"
FILES='paper-agent-native native-oracle-client.mjs native-oracle-server.mjs native-oracle-service.sh native-selection-prepare.sh native-selection-write.sh rich-document.mjs rich-document.test.mjs image-generate.mjs image-generate.test.mjs'

test -x /home/root/node/bin/node
test -x /home/root/node/bin/pi
test -f /home/root/.pi/agent/auth.json
test -d "$QMD_HOME"
for dep in framebuffer-spy.so rm-shot-aarch64.so qt-command-executor.so; do
  test -f "/home/root/xovi/extensions.d/$dep"
done

cleanup() {
  rm -rf "$INCOMING"
  rm -f /tmp/paper-agent-payload.tar.gz /tmp/paper-agent-health.log
}
trap cleanup EXIT
mkdir -p "$INCOMING" "$STATE_ROOT" "$BACKUP" "$NATIVE" "$BASE/selection"
tar -xzf /tmp/paper-agent-payload.tar.gz -C "$INCOMING"
rm -f /tmp/paper-agent-payload.tar.gz

backup_file() {
  name=$1
  path=$2
  if [ -f "$path" ]; then
    cp -p "$path" "$BACKUP/$name"
    echo present > "$BACKUP/$name.state"
  else
    echo absent > "$BACKUP/$name.state"
  fi
}
restore_file() {
  name=$1
  path=$2
  if [ "$(cat "$BACKUP/$name.state")" = present ]; then
    cp -p "$BACKUP/$name" "$path"
  else
    rm -f "$path"
  fi
}

for name in $FILES; do backup_file "$name" "$NATIVE/$name"; done
backup_file paper-agent-native-oracle.service "$UNIT"
backup_file paperAgentSelection.qmd "$QMD_FILE"
backup_file config.env "$CONFIG"
OLD_ENABLED=$(systemctl is-enabled paper-agent-native-oracle.service 2>/dev/null || true)
OLD_ACTIVE=$(systemctl is-active paper-agent-native-oracle.service 2>/dev/null || true)
printf '%s\n' "$OLD_ENABLED" > "$BACKUP/service.enabled"
printf '%s\n' "$OLD_ACTIVE" > "$BACKUP/service.active"

rollback() {
  systemctl stop paper-agent-native-oracle.service 2>/dev/null || true
  for name in $FILES; do restore_file "$name" "$NATIVE/$name"; done
  restore_file paper-agent-native-oracle.service "$UNIT"
  restore_file paperAgentSelection.qmd "$QMD_FILE"
  restore_file config.env "$CONFIG"
  systemctl daemon-reload || true
  if [ "$(cat "$BACKUP/service.enabled")" = enabled ]; then
    systemctl enable paper-agent-native-oracle.service >/dev/null 2>&1 || true
  else
    systemctl disable paper-agent-native-oracle.service >/dev/null 2>&1 || true
  fi
  if [ "$(cat "$BACKUP/service.active")" = active ]; then
    systemctl start paper-agent-native-oracle.service || true
  fi
  /home/root/xovi/start || true
}

cp "$INCOMING/paper-agent-native" "$NATIVE/paper-agent-native"
for name in native-oracle-client.mjs native-oracle-server.mjs native-oracle-service.sh native-selection-prepare.sh native-selection-write.sh rich-document.mjs rich-document.test.mjs image-generate.mjs image-generate.test.mjs; do
  cp "$INCOMING/runtime/$name" "$NATIVE/$name"
done
cp "$INCOMING/paper-agent-native-oracle.service" "$UNIT"
cp "$INCOMING/paper-agent-selection.qmd" "$QMD_FILE"
if [ ! -f "$CONFIG" ]; then cp "$INCOMING/paper-agent.env.example" "$CONFIG"; fi
chmod 0755 "$NATIVE/paper-agent-native" "$NATIVE/native-oracle-service.sh" "$NATIVE/native-selection-prepare.sh" "$NATIVE/native-selection-write.sh"
chmod 0644 "$NATIVE/"*.mjs "$UNIT" "$QMD_FILE" "$CONFIG"
chmod 0600 "$CONFIG"
mkdir -p "$NATIVE/jobs" "$NATIVE/oracle-data" "$NATIVE/artifacts"
chmod 0700 "$NATIVE/jobs" "$NATIVE/oracle-data" "$NATIVE/artifacts" "$BASE/selection"

if ! systemctl daemon-reload || ! systemctl enable paper-agent-native-oracle.service >/dev/null || ! systemctl restart paper-agent-native-oracle.service; then
  rollback
  echo "Paper Agent install rolled back: oracle service failed" >&2
  exit 1
fi

ready=0
attempt=0
while [ "$attempt" -lt 30 ]; do
  if /home/root/node/bin/node "$NATIVE/native-oracle-client.mjs" --health >/tmp/paper-agent-health.log 2>&1; then
    ready=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$ready" -ne 1 ] || [ "$(stat -c %a /run/paper-agent-native-oracle.sock 2>/dev/null || true)" != 600 ]; then
  cat /tmp/paper-agent-health.log >&2 || true
  rollback
  echo "Paper Agent install rolled back: oracle health check failed" >&2
  exit 1
fi

START=$(date +%s)
if ! /home/root/xovi/start; then
  rollback
  echo "Paper Agent install rolled back: Xochitl failed" >&2
  exit 1
fi
sleep 12
LOG=$(journalctl -u xochitl --since "@$START" --no-pager -o cat)
if ! systemctl is-active --quiet xochitl \
  || printf '%s\n' "$LOG" | grep -Eq 'paperAgentSelection\.qmd.*Error|Cannot assign to non-existent property "onPaperAgent|Application is quitting' \
  || ! printf '%s\n' "$LOG" | grep -q '\[qmldiff\].*Loading file paperAgentSelection\.qmd'; then
  rollback
  echo "Paper Agent install rolled back: QMD validation failed" >&2
  exit 1
fi

echo "$BACKUP" > "$BASE/install-last-backup"
echo "paper_agent=installed"
cat /tmp/paper-agent-health.log
echo "xochitl=active"
DEVICE_SCRIPT

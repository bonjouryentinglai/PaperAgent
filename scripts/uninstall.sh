#!/usr/bin/env bash
set -euo pipefail

# Remove Paper Agent's QMD, service, Settings app, hooks, plugin and native
# runtime. The openai-codex login is removed; other Pi provider credentials,
# Node, XOVI/AppLoad, optional XOVI dependencies, backups and user
# configuration are preserved.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"

ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -eu
BASE=/home/root/paper-agent
NATIVE="$BASE/native"
SYSTEMD_HOME="$BASE/systemd"
QMD=/home/root/xovi/exthome/qt-resource-rebuilder/paperAgentSelection.qmd
UNIT_SOURCE="$SYSTEMD_HOME/paper-agent-native-oracle.service"
UNIT_RUN=/run/systemd/system/paper-agent-native-oracle.service
UNIT_LEGACY=/etc/systemd/system/paper-agent-native-oracle.service
START_HOOK=/home/root/xovi/scripts/post-start/paper-agent-native-oracle.sh
IMAGE_PLUGIN=/home/root/xovi/extensions.d/paper-agent-image.so
SETTINGS_APP=/home/root/xovi/exthome/appload/paper-agent-settings
STATE_ROOT="$BASE/backups"
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$STATE_ROOT/uninstall-$STAMP"
mkdir -p "$BACKUP"

AUTH=/home/root/.pi/agent/auth.json
AUTH_TMP="${AUTH}.paper-agent-uninstall.$$"
chatgpt_state=openai-codex-not-present
if [ -f "$AUTH" ]; then
  if [ ! -x /home/root/node/bin/node ]; then
    echo "Cannot safely sign out ChatGPT because the Pi Node runtime is missing" >&2
    exit 1
  fi
  if /home/root/node/bin/node -e '
const fs = require("fs");
const source = process.argv[1];
const target = process.argv[2];
const value = JSON.parse(fs.readFileSync(source, "utf8"));
if (!value || typeof value !== "object" || Array.isArray(value)) process.exit(2);
if (!Object.prototype.hasOwnProperty.call(value, "openai-codex")) process.exit(4);
delete value["openai-codex"];
if (Object.keys(value).length === 0) process.exit(3);
fs.writeFileSync(target, JSON.stringify(value, null, 2) + "\n", { mode: 0o600 });
' "$AUTH" "$AUTH_TMP"; then
    auth_status=0
  else
    auth_status=$?
  fi
  case "$auth_status" in
    0)
      chmod 0600 "$AUTH_TMP"
      mv -f "$AUTH_TMP" "$AUTH"
      chatgpt_state=openai-codex-removed
      ;;
    3)
      rm -f "$AUTH" "$AUTH_TMP"
      rmdir /home/root/.pi/agent /home/root/.pi 2>/dev/null || true
      chatgpt_state=openai-codex-removed
      ;;
    4)
      rm -f "$AUTH_TMP"
      ;;
    *)
      rm -f "$AUTH_TMP"
      echo "Cannot safely remove the openai-codex credential" >&2
      exit 1
      ;;
  esac
fi

systemctl stop paper-agent-native-oracle.service 2>/dev/null || true
systemctl disable paper-agent-native-oracle.service >/dev/null 2>&1 || true
[ -f "$QMD" ] && cp -p "$QMD" "$BACKUP/paperAgentSelection.qmd"
[ -f "$UNIT_SOURCE" ] && cp -p "$UNIT_SOURCE" "$BACKUP/paper-agent-native-oracle.source"
[ -f "$START_HOOK" ] && cp -p "$START_HOOK" "$BACKUP/paper-agent-native-oracle.hook"
[ -f "$IMAGE_PLUGIN" ] && cp -p "$IMAGE_PLUGIN" "$BACKUP/paper-agent-image.so"
[ -f "$BASE/VERSION" ] && cp -p "$BASE/VERSION" "$BACKUP/VERSION"
[ -f "$BASE/activation-required" ] && cp -p "$BASE/activation-required" "$BACKUP/activation-required"
[ -d "$SETTINGS_APP" ] && cp -a "$SETTINGS_APP" "$BACKUP/paper-agent-settings"
if [ -d "$NATIVE" ]; then
  mkdir -p "$BACKUP/native"
  for entry in "$NATIVE"/*; do
    [ -e "$entry" ] || continue
    case "$(basename "$entry")" in
      jobs|oracle-data|artifacts) ;;
      *) cp -a "$entry" "$BACKUP/native/" ;;
    esac
  done
fi
[ -d "$BASE/assets" ] && cp -a "$BASE/assets" "$BACKUP/assets"
rm -f "$QMD" "$UNIT_SOURCE" "$UNIT_RUN" "$UNIT_LEGACY" "$START_HOOK" "$IMAGE_PLUGIN" \
  "$BASE/VERSION" "$BASE/activation-required"
rm -rf "$SETTINGS_APP"
rm -rf "$NATIVE" "$BASE/assets" "$BASE/selection"
rmdir "$SYSTEMD_HOME" 2>/dev/null || true
rm -f /run/paper-agent-native-oracle.sock
rm -rf /run/paper-agent-native-coordinator.lock
rm -f /run/paper-agent-settings-apply.lock
systemctl daemon-reload
/home/root/xovi/start
if systemctl is-active --quiet paper-agent-native-oracle.service; then
  echo "Paper Agent uninstall failed: oracle service is still active" >&2
  exit 1
fi
echo "paper_agent=removed"
echo "preserved_config=$BASE/config.env"
echo "preserved_backups=$STATE_ROOT"
echo "preserved_runtime=$BASE/runtime"
echo "chatgpt=$chatgpt_state"
DEVICE_SCRIPT

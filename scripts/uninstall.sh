#!/usr/bin/env bash
set -euo pipefail

# Remove Paper Agent's QMD, service and native runtime. Pi login, Node, XOVI,
# optional XOVI dependencies, backups and user configuration are preserved.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"

ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -eu
BASE=/home/root/paper-agent
NATIVE="$BASE/native"
QMD=/home/root/xovi/exthome/qt-resource-rebuilder/paperAgentSelection.qmd
UNIT=/etc/systemd/system/paper-agent-native-oracle.service
STATE_ROOT="$BASE/backups"
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$STATE_ROOT/uninstall-$STAMP"
mkdir -p "$BACKUP"

systemctl stop paper-agent-native-oracle.service 2>/dev/null || true
systemctl disable paper-agent-native-oracle.service >/dev/null 2>&1 || true
[ -f "$QMD" ] && cp -p "$QMD" "$BACKUP/paperAgentSelection.qmd"
[ -f "$UNIT" ] && cp -p "$UNIT" "$BACKUP/paper-agent-native-oracle.service"
[ -d "$NATIVE" ] && cp -a "$NATIVE" "$BACKUP/native"
[ -d "$BASE/assets" ] && cp -a "$BASE/assets" "$BACKUP/assets"
rm -f "$QMD" "$UNIT"
rm -rf "$NATIVE" "$BASE/assets"
systemctl daemon-reload
/home/root/xovi/start
echo "paper_agent=removed"
echo "preserved_config=$BASE/config.env"
echo "preserved_backups=$STATE_ROOT"
DEVICE_SCRIPT

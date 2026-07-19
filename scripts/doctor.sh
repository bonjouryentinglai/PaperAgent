#!/usr/bin/env bash
set -euo pipefail

# Read-only installation diagnostics. Credential contents are never printed.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"

ssh -o BatchMode=yes "$DEVICE" 'sh -s' <<'DEVICE_SCRIPT'
set -u
echo "machine=$(cat /sys/devices/soc0/machine 2>/dev/null || echo unknown)"
echo "os=$(cat /etc/os-release 2>/dev/null | sed -n "s/^VERSION_ID=//p" | tr -d "\"")"
echo "xochitl=$(systemctl is-active xochitl 2>/dev/null || true)"
echo "oracle=$(systemctl is-active paper-agent-native-oracle.service 2>/dev/null || true)"
echo "qmd=$([ -f /home/root/xovi/exthome/qt-resource-rebuilder/paperAgentSelection.qmd ] && echo present || echo missing)"
echo "icons=$([ -f /home/root/paper-agent/assets/icons/paper-agent-ai.svg ] && [ -f /home/root/paper-agent/assets/icons/paper-agent-beautify.svg ] && echo present || echo missing)"
echo "credential=$([ -f /home/root/.pi/agent/auth.json ] && echo present || echo missing)"
echo "socket_mode=$(stat -c %a /run/paper-agent-native-oracle.sock 2>/dev/null || echo missing)"
if [ -f /home/root/paper-agent/config.env ]; then
  grep -E '^PAPER_AGENT_(PROVIDER|MODEL|THINKING|CJK_SCALE|IMAGE_(RESPONSES_MODEL|MODEL|SIZE|QUALITY|MAX_WIDTH|MAX_HEIGHT))=' /home/root/paper-agent/config.env || true
fi
if [ -x /home/root/paper-agent/native/paper-agent-native ]; then
  /home/root/paper-agent/native/paper-agent-native --version
  /home/root/paper-agent/native/paper-agent-native probe || true
fi
if [ -f /home/root/paper-agent/native/native-oracle-client.mjs ]; then
  /home/root/node/bin/node /home/root/paper-agent/native/native-oracle-client.mjs --health || true
fi
DEVICE_SCRIPT

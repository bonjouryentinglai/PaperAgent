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
echo "oracle_source=$([ -f /home/root/paper-agent/systemd/paper-agent-native-oracle.service ] && echo present || echo missing)"
echo "xovi_start_hook=$([ -x /home/root/xovi/scripts/post-start/paper-agent-native-oracle.sh ] && echo present || echo missing)"
echo "qmd=$([ -f /home/root/xovi/exthome/qt-resource-rebuilder/paperAgentSelection.qmd ] && echo present || echo missing)"
echo "icons=$([ -f /home/root/paper-agent/assets/icons/paper-agent-ai.svg ] && [ -f /home/root/paper-agent/assets/icons/paper-agent-beautify.svg ] && echo present || echo missing)"
echo "image_plugin=$([ -f /home/root/xovi/extensions.d/paper-agent-image.so ] && echo present || echo missing)"
echo "phase_2a_runtime=$([ -f /home/root/paper-agent/native/scene.mjs ] && [ -f /home/root/paper-agent/native/paper-agent-tools.ts ] && echo present || echo missing)"
echo "phase_2a_skills=$([ -f /home/root/paper-agent/native/skills/ai-selection/SKILL.md ] && [ -f /home/root/paper-agent/native/skills/structured-drawing/SKILL.md ] && [ -f /home/root/paper-agent/native/skills/beautify-selection/SKILL.md ] && echo present || echo missing)"
echo "settings_controller=$([ -f /home/root/paper-agent/native/settings-controller.mjs ] && echo present || echo missing)"
echo "settings_app=$([ -f /home/root/xovi/exthome/appload/paper-agent-settings/manifest.json ] && [ -x /home/root/xovi/exthome/appload/paper-agent-settings/backend/entry ] && echo present || echo missing)"
echo "credential=$([ -f /home/root/.pi/agent/auth.json ] && echo present || echo missing)"
echo "socket_mode=$(stat -c %a /run/paper-agent-native-oracle.sock 2>/dev/null || echo missing)"
echo "coordinator_lock=$([ -d /run/paper-agent-native-coordinator.lock ] && echo active || echo clear)"
X_PID=$(systemctl show xochitl -p MainPID --value 2>/dev/null || true)
if [ -n "$X_PID" ] && [ "$X_PID" != 0 ]; then
  QMD_LOG=$(journalctl "_PID=$X_PID" -n 1000 --no-pager -o cat 2>/dev/null || true)
  if printf '%s\n' "$QMD_LOG" | grep -Eq 'Error while processing file tree:.*paperAgentSelection\.qmd|paperAgentSelection\.qmd.*(Cannot locate element in tree|Error|failed)|Cannot assign to non-existent property "onPaperAgent|CommandExecutor is not a type'; then
    echo "qmd_runtime=error"
    printf '%s\n' "$QMD_LOG" | grep -E 'paperAgentSelection\.qmd|Cannot assign to non-existent property "onPaperAgent' | tail -n 5
  elif printf '%s\n' "$QMD_LOG" | grep -q '\[qmldiff\].*Loading file paperAgentSelection\.qmd'; then
    echo "qmd_runtime=healthy"
  else
    echo "qmd_runtime=unverified"
  fi
else
  echo "qmd_runtime=unavailable"
fi
if [ -f /home/root/paper-agent/config.env ]; then
  grep -E '^PAPER_AGENT_(PROVIDER|MODEL|THINKING|TEXT_SCALE_PERCENT|MIN_AUTO_SCALE_PERCENT|CJK_SCALE|IMAGE_(RESPONSES_MODEL|MODEL|SIZE|QUALITY|MAX_WIDTH|MAX_HEIGHT))=' /home/root/paper-agent/config.env || true
fi
if [ -x /home/root/paper-agent/native/paper-agent-native ]; then
  /home/root/paper-agent/native/paper-agent-native --version
  /home/root/paper-agent/native/paper-agent-native probe || true
fi
if [ -f /home/root/paper-agent/native/native-oracle-client.mjs ]; then
  /home/root/node/bin/node /home/root/paper-agent/native/native-oracle-client.mjs --health || true
fi
DEVICE_SCRIPT

// SPDX-License-Identifier: MIT
package maintenance

import (
	"fmt"
	"strings"
)

type Runner interface {
	Run(command string) (string, error)
}

// UninstallCommand intentionally preserves shared XOVI/AppLoad components,
// the Pi/Node runtime, ChatGPT credentials, user configuration, and backups.
// Keep this behavior aligned with scripts/uninstall.sh.
const UninstallCommand = `set -eu
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

systemctl stop paper-agent-native-oracle.service 2>/dev/null || true
systemctl disable paper-agent-native-oracle.service >/dev/null 2>&1 || true
[ -f "$QMD" ] && cp -p "$QMD" "$BACKUP/paperAgentSelection.qmd"
[ -f "$UNIT_SOURCE" ] && cp -p "$UNIT_SOURCE" "$BACKUP/paper-agent-native-oracle.source"
[ -f "$START_HOOK" ] && cp -p "$START_HOOK" "$BACKUP/paper-agent-native-oracle.hook"
[ -f "$IMAGE_PLUGIN" ] && cp -p "$IMAGE_PLUGIN" "$BACKUP/paper-agent-image.so"
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
rm -f "$QMD" "$UNIT_SOURCE" "$UNIT_RUN" "$UNIT_LEGACY" "$START_HOOK" "$IMAGE_PLUGIN"
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
echo "preserved_credential=/home/root/.pi/agent/auth.json"
`

func Uninstall(device Runner) (string, error) {
	output, err := device.Run(UninstallCommand)
	if err != nil {
		return output, fmt.Errorf("remove Paper Agent: %w: %s", err, strings.TrimSpace(output))
	}
	if !strings.Contains(output, "paper_agent=removed") {
		return output, fmt.Errorf("remove Paper Agent: device did not confirm removal")
	}
	return output, nil
}

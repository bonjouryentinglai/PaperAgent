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

// FullCleanupCommand removes only components recorded at installation time as
// created by Paper Agent Installer. It intentionally refuses to infer
// ownership from file locations alone.
const FullCleanupCommand = `set -eu
BASE=/home/root/paper-agent
OWN="$BASE/installer-owned"
test -d "$OWN" || {
  echo "No installer ownership record is available; refusing full cleanup" >&2
  exit 1
}

owned=0
for component in xovi appload runtime pi-packages persistence oauth \
  extension-framebuffer-spy extension-qt-command-executor \
  extension-xovi-message-broker extension-rm-shot; do
  if [ -e "$OWN/$component" ] || [ -e "$OWN/$component.sha256" ]; then
    owned=1
  fi
done
[ "$owned" -eq 1 ] || {
  echo "Installer ownership record is empty; refusing full cleanup" >&2
  exit 1
}

removed=
append_removed() {
  if [ -n "$removed" ]; then removed="$removed,$1"; else removed="$1"; fi
}

if [ -e "$OWN/oauth.sha256" ] && [ -f /home/root/.pi/agent/auth.json ]; then
  expected=$(cat "$OWN/oauth.sha256")
  actual=$(sha256sum /home/root/.pi/agent/auth.json | awk '{print $1}')
  if [ "$expected" = "$actual" ]; then
    rm -f /home/root/.pi/agent/auth.json
    rmdir /home/root/.pi/agent /home/root/.pi 2>/dev/null || true
    append_removed oauth
  else
    echo "preserved_changed_oauth=1"
  fi
fi

if [ -e "$OWN/pi-packages" ] && [ ! -e "$OWN/runtime" ] && [ -x /home/root/node/bin/npm ]; then
  HOME=/home/root PATH="/home/root/node/bin:$PATH" \
    /home/root/node/bin/npm uninstall --global --prefix /home/root/node \
    @earendil-works/pi-coding-agent @earendil-works/pi-ai \
    --no-audit --no-fund >/dev/null 2>&1
  append_removed pi-packages
fi

if [ -e "$OWN/runtime" ]; then
  if [ -L /home/root/node ]; then
    case "$(readlink /home/root/node)" in
      "$BASE/runtime/"*) rm -f /home/root/node ;;
    esac
  fi
  rm -rf "$BASE/runtime"
  append_removed runtime
fi

if [ -e "$OWN/persistence" ]; then
  systemctl stop xovi-tripletap 2>/dev/null || true
  systemctl disable xovi-tripletap >/dev/null 2>&1 || true
  mount -o remount,rw / 2>/dev/null || true
  rm -f /etc/systemd/system/xovi-tripletap.service
  systemctl daemon-reload
  mount -o remount,ro / 2>/dev/null || true
  rm -rf /home/root/xovi-tripletap
  append_removed persistence
fi

restart_xochitl=0
if [ -e "$OWN/xovi" ]; then
  systemctl stop xochitl 2>/dev/null || true
  rm -rf /home/root/xovi
  systemctl start xochitl
  append_removed xovi
else
  if [ -e "$OWN/appload" ]; then
    restart_xochitl=1
    rm -f /home/root/xovi/extensions.d/appload.so
    rm -rf /home/root/xovi/exthome/appload
    append_removed appload
  fi
  if [ -e "$OWN/extension-framebuffer-spy" ]; then
    restart_xochitl=1
    rm -f /home/root/xovi/extensions.d/framebuffer-spy.so
    append_removed extension-framebuffer-spy
  fi
  if [ -e "$OWN/extension-qt-command-executor" ]; then
    restart_xochitl=1
    rm -f /home/root/xovi/extensions.d/qt-command-executor.so
    append_removed extension-qt-command-executor
  fi
  if [ -e "$OWN/extension-xovi-message-broker" ]; then
    restart_xochitl=1
    rm -f /home/root/xovi/extensions.d/xovi-message-broker.so
    append_removed extension-xovi-message-broker
  fi
  if [ -e "$OWN/extension-rm-shot" ]; then
    restart_xochitl=1
    rm -f /home/root/xovi/extensions.d/rm-shot-aarch64.so
    append_removed extension-rm-shot
  fi
  if [ "$restart_xochitl" -eq 1 ]; then
    systemctl stop xochitl 2>/dev/null || true
    if [ -x /home/root/xovi/start ]; then
      /home/root/xovi/start
    else
      systemctl start xochitl
    fi
  fi
fi

rm -rf "$BASE"
echo "removed_components=$removed"
echo "full_cleanup=removed"
`

func FullCleanup(device Runner) (string, error) {
	output, err := device.Run(FullCleanupCommand)
	if err != nil {
		return output, fmt.Errorf("remove installer-managed components: %w: %s", err, strings.TrimSpace(output))
	}
	if !strings.Contains(output, "full_cleanup=removed") {
		return output, fmt.Errorf("remove installer-managed components: device did not confirm cleanup")
	}
	return output, nil
}

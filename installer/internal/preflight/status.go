// SPDX-License-Identifier: MIT
package preflight

import (
	"fmt"
	"strconv"
	"strings"
)

type Runner interface {
	Run(command string) (string, error)
}

type Status struct {
	Address          string   `json:"address"`
	Model            string   `json:"model"`
	OSVersion        string   `json:"osVersion"`
	FreeSpaceKB      int64    `json:"freeSpaceKB"`
	InstalledVersion string   `json:"installedVersion"`
	SupportedModel   bool     `json:"supportedModel"`
	DeveloperMode    bool     `json:"developerMode"`
	XOVIInstalled    bool     `json:"xoviInstalled"`
	AppLoadInstalled bool     `json:"appLoadInstalled"`
	QMLIndex         bool     `json:"qmlIndex"`
	XOVIPersistence  bool     `json:"xoviPersistence"`
	NodeInstalled    bool     `json:"nodeInstalled"`
	PiInstalled      bool     `json:"piInstalled"`
	NativeDeps       bool     `json:"nativeDeps"`
	ChatGPTLoggedIn  bool     `json:"chatGPTLoggedIn"`
	PaperAgent       bool     `json:"paperAgent"`
	ServiceActive    bool     `json:"serviceActive"`
	SettingsApp      bool     `json:"settingsApp"`
	InstallerOwned   []string `json:"installerOwned"`
}

const inspectCommand = `set -u
printf 'model='; cat /proc/device-tree/model 2>/dev/null | tr -d '\0'; printf '\n'
printf 'os='; (. /etc/os-release 2>/dev/null; printf '%s' "${IMG_VERSION:-unknown}"); printf '\n'
printf 'free_kb='; df -k /home 2>/dev/null | tail -n1 | awk '{print $(NF-2)}'
test -d /home/root/xovi && echo 'xovi=1' || echo 'xovi=0'
test -f /home/root/xovi/extensions.d/appload.so && echo 'appload=1' || echo 'appload=0'
test -s /home/root/xovi/exthome/qt-resource-rebuilder/hashtab && echo 'qml_index=1' || echo 'qml_index=0'
(systemctl is-enabled --quiet xovi-tripletap 2>/dev/null \
  || systemctl is-enabled --quiet xovi-always-on 2>/dev/null \
  || systemctl is-enabled --quiet xovi-boot 2>/dev/null) \
  && echo 'xovi_persistence=1' || echo 'xovi_persistence=0'
test -x /home/root/node/bin/node && echo 'node=1' || echo 'node=0'
test -x /home/root/node/bin/pi-ai && test -x /home/root/node/bin/pi && echo 'pi=1' || echo 'pi=0'
test -f /home/root/xovi/extensions.d/framebuffer-spy.so \
  && test -f /home/root/xovi/extensions.d/rm-shot-aarch64.so \
  && test -f /home/root/xovi/extensions.d/qt-command-executor.so \
  && test -f /home/root/xovi/extensions.d/xovi-message-broker.so \
  && echo 'native_deps=1' || echo 'native_deps=0'
if test -x /home/root/node/bin/node && test -s /home/root/.pi/agent/auth.json; then
  /home/root/node/bin/node -e '
    const fs=require("fs");
    const value=JSON.parse(fs.readFileSync("/home/root/.pi/agent/auth.json","utf8"));
    const auth=value["openai-codex"];
    process.exit(auth && auth.type === "oauth" && auth.access && auth.refresh ? 0 : 1);
  ' >/dev/null 2>&1 && echo 'login=1' || echo 'login=0'
else
  echo 'login=0'
fi
test -x /home/root/paper-agent/native/paper-agent-native && echo 'paper_agent=1' || echo 'paper_agent=0'
systemctl is-active --quiet paper-agent-native-oracle.service && echo 'service=1' || echo 'service=0'
test -f /home/root/xovi/exthome/appload/paper-agent-settings/manifest.json && echo 'settings_app=1' || echo 'settings_app=0'
OWN=/home/root/paper-agent/installer-owned
owned=
for component in xovi appload runtime pi-packages persistence oauth \
  extension-framebuffer-spy extension-qt-command-executor \
  extension-xovi-message-broker extension-rm-shot; do
  if test -e "$OWN/$component" || test -e "$OWN/$component.sha256"; then
    if test -n "$owned"; then owned="$owned,$component"; else owned="$component"; fi
  fi
done
printf 'installer_owned=%s\n' "$owned"
printf 'installed_version='; cat /home/root/paper-agent/VERSION 2>/dev/null || true; printf '\n'
`

func supportedModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "chiappa")
}

func parse(output, address string) (Status, error) {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			values[key] = value
		}
	}
	if strings.TrimSpace(values["model"]) == "" {
		return Status{}, fmt.Errorf("device returned no model")
	}
	freeSpace, _ := strconv.ParseInt(values["free_kb"], 10, 64)
	model := strings.TrimSpace(values["model"])
	supported := supportedModel(model)
	var installerOwned []string
	for _, component := range strings.Split(values["installer_owned"], ",") {
		component = strings.TrimSpace(component)
		if component != "" {
			installerOwned = append(installerOwned, component)
		}
	}
	return Status{
		Address:          address,
		Model:            model,
		OSVersion:        strings.TrimSpace(values["os"]),
		FreeSpaceKB:      freeSpace,
		InstalledVersion: strings.TrimSpace(values["installed_version"]),
		SupportedModel:   supported,
		DeveloperMode:    true, // A successful root SSH session is the gate.
		XOVIInstalled:    values["xovi"] == "1",
		AppLoadInstalled: values["appload"] == "1",
		QMLIndex:         values["qml_index"] == "1",
		XOVIPersistence:  values["xovi_persistence"] == "1",
		NodeInstalled:    values["node"] == "1",
		PiInstalled:      values["pi"] == "1",
		NativeDeps:       values["native_deps"] == "1",
		ChatGPTLoggedIn:  values["login"] == "1",
		PaperAgent:       values["paper_agent"] == "1",
		ServiceActive:    values["service"] == "1",
		SettingsApp:      values["settings_app"] == "1",
		InstallerOwned:   installerOwned,
	}, nil
}

func Inspect(device Runner, address string) (Status, error) {
	output, err := device.Run(inspectCommand)
	if err != nil {
		return Status{}, fmt.Errorf("read device status: %w: %s", err, strings.TrimSpace(output))
	}
	return parse(output, address)
}

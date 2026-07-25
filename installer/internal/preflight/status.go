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
	Address          string `json:"address"`
	Model            string `json:"model"`
	OSVersion        string `json:"osVersion"`
	FreeSpaceKB      int64  `json:"freeSpaceKB"`
	SupportedModel   bool   `json:"supportedModel"`
	DeveloperMode    bool   `json:"developerMode"`
	XOVIInstalled    bool   `json:"xoviInstalled"`
	AppLoadInstalled bool   `json:"appLoadInstalled"`
	ChatGPTLoggedIn  bool   `json:"chatGPTLoggedIn"`
	PaperAgent       bool   `json:"paperAgent"`
	ServiceActive    bool   `json:"serviceActive"`
	SettingsApp      bool   `json:"settingsApp"`
}

const inspectCommand = `set -u
printf 'model='; cat /proc/device-tree/model 2>/dev/null | tr -d '\0'; printf '\n'
printf 'os='; (. /etc/os-release 2>/dev/null; printf '%s' "${IMG_VERSION:-unknown}"); printf '\n'
printf 'free_kb='; df -k /home 2>/dev/null | tail -n1 | awk '{print $(NF-2)}'
test -d /home/root/xovi && echo 'xovi=1' || echo 'xovi=0'
test -f /home/root/xovi/extensions.d/appload.so && echo 'appload=1' || echo 'appload=0'
test -f /home/root/.pi/agent/auth.json && echo 'login=1' || echo 'login=0'
test -x /home/root/paper-agent/native/paper-agent-native && echo 'paper_agent=1' || echo 'paper_agent=0'
systemctl is-active --quiet paper-agent-native-oracle.service && echo 'service=1' || echo 'service=0'
test -f /home/root/xovi/exthome/appload/paper-agent-settings/manifest.json && echo 'settings_app=1' || echo 'settings_app=0'
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
	return Status{
		Address:          address,
		Model:            model,
		OSVersion:        strings.TrimSpace(values["os"]),
		FreeSpaceKB:      freeSpace,
		SupportedModel:   supported,
		DeveloperMode:    true, // A successful root SSH session is the gate.
		XOVIInstalled:    values["xovi"] == "1",
		AppLoadInstalled: values["appload"] == "1",
		ChatGPTLoggedIn:  values["login"] == "1",
		PaperAgent:       values["paper_agent"] == "1",
		ServiceActive:    values["service"] == "1",
		SettingsApp:      values["settings_app"] == "1",
	}, nil
}

func Inspect(device Runner, address string) (Status, error) {
	output, err := device.Run(inspectCommand)
	if err != nil {
		return Status{}, fmt.Errorf("read device status: %w: %s", err, strings.TrimSpace(output))
	}
	return parse(output, address)
}

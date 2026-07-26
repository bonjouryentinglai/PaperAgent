// SPDX-License-Identifier: MIT
package oauth

import (
	"os/exec"
	"strings"
	"testing"
)

func TestUpdateOutputExtractsDeviceCode(t *testing.T) {
	manager := NewManager()
	manager.state.Running = true
	manager.updateOutput("\x1b[32mOpen this URL in your browser:\x1b[0m\nhttps://auth.openai.com/codex/device\n")
	manager.updateOutput("Enter code: ABCD-EFGH\n")
	state := manager.State()
	if state.URL != "https://auth.openai.com/codex/device" {
		t.Fatalf("URL = %q", state.URL)
	}
	if state.Code != "ABCD-EFGH" {
		t.Fatalf("code = %q", state.Code)
	}
	if !state.Running {
		t.Fatal("parser changed running state")
	}
}

func TestLoginCommandRecordsOnlyItsOwnForegroundProcess(t *testing.T) {
	const pidFile = "/run/paper-agent-installer-login-test.pid"
	command := loginCommand(pidFile)
	for _, required := range []string{
		"PIDFILE=" + pidFile,
		`printf "%s\n" "$$" >"$PIDFILE"`,
		"exec env HOME=/home/root",
		"/home/root/node/bin/pi-ai login openai-codex",
		`rm -f "$PIDFILE"`,
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("login command is missing %q", required)
		}
	}
	checkShellSyntax(t, "login", command)
}

func TestCancelCommandTargetsOnlyRecordedOpenAICodexLogin(t *testing.T) {
	const pidFile = "/run/paper-agent-installer-login-test.pid"
	command := cancelLoginCommand(pidFile)
	for _, required := range []string{
		"PIDFILE=" + pidFile,
		`command=$(tr '\000' ' ' <"/proc/$pid/cmdline")`,
		"node /home/root/node/bin/pi-ai login openai-codex ",
		`kill -TERM "$pid"`,
		`rm -f "$PIDFILE"`,
		"login_process=stopped",
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("cancel command is missing %q", required)
		}
	}
	for _, forbidden := range []string{"pkill", "killall", "pi-ai login\""} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("cancel command contains broad process match %q", forbidden)
		}
	}
	checkShellSyntax(t, "cancel", command)
}

func checkShellSyntax(t *testing.T, name, script string) {
	t.Helper()
	command := exec.Command("sh", "-n")
	command.Stdin = strings.NewReader(script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s shell syntax: %v: %s", name, err, output)
	}
}

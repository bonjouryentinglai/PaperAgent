// SPDX-License-Identifier: MIT
package maintenance

import (
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	output  string
	err     error
	command string
}

func (f *fakeRunner) Run(command string) (string, error) {
	f.command = command
	return f.output, f.err
}

func TestUninstallPreservesSharedComponentsAndCredentials(t *testing.T) {
	runner := &fakeRunner{output: `paper_agent=removed
preserved_config=/home/root/paper-agent/config.env
preserved_runtime=/home/root/paper-agent/runtime
preserved_credential=/home/root/.pi/agent/auth.json
`}
	if _, err := Uninstall(runner); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"rm -rf /home/root/xovi",
		"rm -f /home/root/.pi/agent/auth.json",
		"rm -rf /home/root/paper-agent/runtime",
		"rm -f /home/root/paper-agent/config.env",
	} {
		if strings.Contains(runner.command, forbidden) {
			t.Fatalf("uninstall contains forbidden cleanup %q", forbidden)
		}
	}
	for _, required := range []string{
		"paperAgentSelection.qmd",
		"paper-agent-native-oracle.service",
		"paper-agent-image.so",
		"paper-agent-settings",
		"paper_agent=removed",
	} {
		if !strings.Contains(runner.command, required) {
			t.Fatalf("uninstall is missing %q", required)
		}
	}
}

func TestUninstallRequiresDeviceConfirmation(t *testing.T) {
	if _, err := Uninstall(&fakeRunner{output: "done\n"}); err == nil {
		t.Fatal("missing removal confirmation was accepted")
	}
	if _, err := Uninstall(&fakeRunner{output: "failed\n", err: errors.New("exit 1")}); err == nil {
		t.Fatal("command failure was accepted")
	}
}

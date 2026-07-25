// SPDX-License-Identifier: MIT
package preflight

import (
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	output string
	err    error
}

func (f fakeRunner) Run(string) (string, error) {
	return f.output, f.err
}

func TestInspectParsesMoveStatus(t *testing.T) {
	status, err := Inspect(fakeRunner{output: `model=reMarkable Chiappa
os=3.22.1
free_kb=123456
xovi=1
appload=1
qml_index=1
xovi_persistence=1
node=1
pi=1
native_deps=1
login=1
paper_agent=1
service=1
settings_app=0
installer_owned=xovi,appload,runtime,oauth
installed_version=0.2.0
`}, "10.11.99.1")
	if err != nil {
		t.Fatal(err)
	}
	if !status.SupportedModel || !status.DeveloperMode || !status.PaperAgent {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.SettingsApp || status.FreeSpaceKB != 123456 {
		t.Fatalf("unexpected settings/storage status: %#v", status)
	}
	if !status.NodeInstalled || !status.PiInstalled || !status.NativeDeps {
		t.Fatalf("expected runtime and native dependencies: %#v", status)
	}
	if !status.QMLIndex || !status.XOVIPersistence {
		t.Fatalf("expected XOVI QML index and persistence: %#v", status)
	}
	if status.InstalledVersion != "0.2.0" {
		t.Fatalf("installed version = %q", status.InstalledVersion)
	}
	if got := strings.Join(status.InstallerOwned, ","); got != "xovi,appload,runtime,oauth" {
		t.Fatalf("installer-owned components = %q", got)
	}
}

func TestInspectRejectsMissingModelAndCommandFailure(t *testing.T) {
	if _, err := Inspect(fakeRunner{output: "os=3.22\n"}, "device"); err == nil {
		t.Fatal("missing model was accepted")
	}
	if _, err := Inspect(fakeRunner{err: errors.New("closed")}, "device"); err == nil {
		t.Fatal("command failure was accepted")
	}
}

func TestInspectDoesNotClaimOtherRemarkableModelsAreSupported(t *testing.T) {
	for _, model := range []string{
		"reMarkable Ferrari",
		"reMarkable Tatsu",
		"reMarkable Paper Pro",
	} {
		status, err := Inspect(fakeRunner{output: "model=" + model + "\nos=3.27\n"}, "device")
		if err != nil {
			t.Fatal(err)
		}
		if status.SupportedModel {
			t.Fatalf("%q was reported as supported", model)
		}
	}
}

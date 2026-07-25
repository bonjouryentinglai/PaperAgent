// SPDX-License-Identifier: MIT
package preflight

import (
	"errors"
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
login=1
paper_agent=1
service=1
settings_app=0
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
}

func TestInspectRejectsMissingModelAndCommandFailure(t *testing.T) {
	if _, err := Inspect(fakeRunner{output: "os=3.22\n"}, "device"); err == nil {
		t.Fatal("missing model was accepted")
	}
	if _, err := Inspect(fakeRunner{err: errors.New("closed")}, "device"); err == nil {
		t.Fatal("command failure was accepted")
	}
}

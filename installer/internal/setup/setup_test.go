// SPDX-License-Identifier: MIT
package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
)

func TestGitHubArchiveEvtestModeDoesNotNeedToBeExecutable(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "evtest.arm64")
	installed := filepath.Join(directory, "evtest")
	if err := os.WriteFile(source, []byte("binary"), 0o664); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 != 0 {
		t.Fatalf("fixture unexpectedly executable: %v", info.Mode())
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, data, 0o755); err != nil {
		t.Fatal(err)
	}
	installedInfo, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if installedInfo.Mode()&0o111 == 0 {
		t.Fatalf("installed evtest is not executable: %v", installedInfo.Mode())
	}

	sourceCheck := `test -s "$SOURCE/evtest.arm64"`
	copyStep := `cp "$SOURCE/evtest.arm64" "$INSTALL/evtest"`
	chmodStep := `chmod 0755 "$INSTALL/evtest"`
	installedCheck := `test -x "$INSTALL/evtest"`
	for _, step := range []string{sourceCheck, copyStep, chmodStep, installedCheck} {
		if !strings.Contains(installTripleTapCommand, step) {
			t.Fatalf("triple-tap installer is missing %q", step)
		}
	}
	if strings.Contains(installTripleTapCommand, `test -x "$SOURCE/evtest.arm64"`) {
		t.Fatal("triple-tap installer still requires the GitHub archive entry to be executable")
	}
	if !(strings.Index(installTripleTapCommand, sourceCheck) < strings.Index(installTripleTapCommand, copyStep) &&
		strings.Index(installTripleTapCommand, copyStep) < strings.Index(installTripleTapCommand, chmodStep) &&
		strings.Index(installTripleTapCommand, chmodStep) < strings.Index(installTripleTapCommand, installedCheck)) {
		t.Fatal("triple-tap evtest validation and installation steps are out of order")
	}

	enableStep := `"$INSTALL/enable.sh"`
	whiteoutCheck := `if [ -c "$whiteout" ]`
	if !strings.Contains(installTripleTapCommand, "/var/volatile/etc/systemd/system/xovi-tripletap.service") ||
		!strings.Contains(installTripleTapCommand, "/var/volatile/etc/systemd/system/multi-user.target.wants/xovi-tripletap.service") {
		t.Fatal("triple-tap installer does not clear the known persistence whiteouts")
	}
	if strings.Index(installTripleTapCommand, enableStep) >= strings.Index(installTripleTapCommand, whiteoutCheck) {
		t.Fatal("persistence whiteouts must be inspected only after enable.sh unmounts /etc")
	}
	if strings.Contains(installTripleTapCommand, `[ -e "$whiteout" ]`) {
		t.Fatal("regular persistence files must not be removed as whiteouts")
	}
}

func TestDeployRejectsUnexpectedBundleNameBeforeDeviceAccess(t *testing.T) {
	if _, err := Deploy(nil, "/tmp/not-the-release.tar.gz"); err == nil {
		t.Fatal("unexpected bundle filename was accepted")
	}
	if _, err := DeployStaged(nil, "/tmp/not-the-release.tar.gz"); err == nil {
		t.Fatal("unexpected staged bundle filename was accepted")
	}
}

func TestTailBoundsDiagnosticOutput(t *testing.T) {
	if got := tail("  abc  ", 10); got != "abc" {
		t.Fatalf("tail = %q", got)
	}
	if got := tail("abcdef", 3); got != "…def" {
		t.Fatalf("bounded tail = %q", got)
	}
}

func TestRepairPreservesDetectedSharedComponents(t *testing.T) {
	status := preflight.Status{
		XOVIInstalled:    true,
		AppLoadInstalled: true,
		NativeDeps:       true,
		QMLIndex:         true,
	}
	plan := planPrerequisites(status, true)
	if plan.xoviArchive {
		t.Fatal("repair would refresh an already detected XOVI installation")
	}
	if plan.appLoad {
		t.Fatal("repair would refresh an already detected AppLoad installation")
	}
	if plan.nativeBridge || !plan.qmlIndex {
		t.Fatal("repair must preserve shared native files and rebuild only Paper Agent's QML index")
	}
}

func TestMissingPrerequisitesNamesOnlyFailedChecks(t *testing.T) {
	status := preflight.Status{
		XOVIInstalled:    true,
		AppLoadInstalled: true,
		NodeInstalled:    true,
		PiInstalled:      false,
		NativeDeps:       false,
		QMLIndex:         true,
		XOVIPersistence:  false,
	}
	want := []string{"Pi", "native selection bridge", "XOVI startup"}
	got := missingPrerequisites(status)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("missingPrerequisites() = %q, want %q", got, want)
	}

	status.PiInstalled = true
	status.NativeDeps = true
	status.XOVIPersistence = true
	if got := missingPrerequisites(status); len(got) != 0 {
		t.Fatalf("complete status reported missing prerequisites: %q", got)
	}
}

// SPDX-License-Identifier: MIT
package setup

import (
	"testing"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
)

func TestDeployRejectsUnexpectedBundleNameBeforeDeviceAccess(t *testing.T) {
	if _, err := Deploy(nil, "/tmp/not-the-release.tar.gz"); err == nil {
		t.Fatal("unexpected bundle filename was accepted")
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

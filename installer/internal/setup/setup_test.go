// SPDX-License-Identifier: MIT
package setup

import "testing"

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

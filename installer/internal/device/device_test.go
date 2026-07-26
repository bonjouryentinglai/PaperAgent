// SPDX-License-Identifier: MIT
package device

import "testing"

func TestProbeClassificationRejectsGenericDropbearHost(t *testing.T) {
	router := &Probe{
		Addr:   "192.168.1.1",
		Banner: "SSH-2.0-dropbear",
	}
	if router.IsPaperPro() {
		t.Fatal("generic Dropbear host must not be classified as a Paper Pro")
	}
	if !router.IsDropbear() {
		t.Fatal("generic Dropbear host should still be recognized as Dropbear")
	}
}

func TestProbeClassificationAcceptsPaperProGate(t *testing.T) {
	move := &Probe{
		Addr:   DefaultUSBAddr,
		Gate:   "reMarkable developer mode",
		Banner: "SSH-2.0-dropbear",
	}
	if !move.IsPaperPro() {
		t.Fatal("developer-mode gate plus Dropbear banner should be classified as a Paper Pro")
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/it's-safe")
	want := "'/tmp/it'\\''s-safe'"
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

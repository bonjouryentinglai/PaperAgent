// SPDX-License-Identifier: MIT
package device

import "testing"

func TestShellQuote(t *testing.T) {
	got := shellQuote("/tmp/it's-safe")
	want := "'/tmp/it'\\''s-safe'"
	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

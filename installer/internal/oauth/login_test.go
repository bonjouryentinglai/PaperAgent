// SPDX-License-Identifier: MIT
package oauth

import "testing"

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

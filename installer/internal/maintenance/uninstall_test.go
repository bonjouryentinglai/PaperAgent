// SPDX-License-Identifier: MIT
package maintenance

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

func TestUninstallPreservesSharedComponentsAndSignsOutChatGPT(t *testing.T) {
	runner := &fakeRunner{output: `paper_agent=removed
preserved_config=/home/root/paper-agent/config.env
preserved_runtime=/home/root/paper-agent/runtime
chatgpt=openai-codex-removed
`}
	if _, err := Uninstall(runner); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"rm -rf /home/root/xovi",
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
		`delete value["openai-codex"]`,
		"chatgpt=$chatgpt_state",
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
	if _, err := Uninstall(&fakeRunner{output: "paper_agent=removed\n"}); err == nil {
		t.Fatal("missing ChatGPT sign-out confirmation was accepted")
	}
	if _, err := Uninstall(&fakeRunner{output: "failed\n", err: errors.New("exit 1")}); err == nil {
		t.Fatal("command failure was accepted")
	}
}

func uninstallCredentialJavaScript(t *testing.T) string {
	t.Helper()
	const prefix = `/home/root/node/bin/node -e '`
	const suffix = `' "$AUTH" "$AUTH_TMP"; then`
	start := strings.Index(SignOutChatGPTCommand, prefix)
	if start < 0 {
		t.Fatal("credential removal JavaScript start was not found")
	}
	after := strings.TrimPrefix(SignOutChatGPTCommand[start:], prefix)
	script, _, found := strings.Cut(after, suffix)
	if !found {
		t.Fatal("credential removal JavaScript end was not found")
	}
	return script
}

func TestSignOutChatGPTRequiresDeviceConfirmation(t *testing.T) {
	for _, runner := range []*fakeRunner{
		{output: "done\n"},
		{output: "failed\n", err: errors.New("exit 1")},
	} {
		if _, err := SignOutChatGPT(runner); err == nil {
			t.Fatal("missing sign-out confirmation was accepted")
		}
	}
	for _, confirmation := range []string{
		"chatgpt=openai-codex-removed\n",
		"chatgpt=openai-codex-not-present\n",
	} {
		if _, err := SignOutChatGPT(&fakeRunner{output: confirmation}); err != nil {
			t.Fatalf("valid sign-out confirmation was rejected: %v", err)
		}
	}
}

func TestUninstallCredentialRemovalPreservesOtherPiProviders(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "auth.json")
	target := filepath.Join(dir, "auth.next.json")
	content := `{
  "openai-codex": {"type":"oauth","access":"secret","refresh":"secret"},
  "another-provider": {"type":"token","value":"keep-me"}
}`
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "-e", uninstallCredentialJavaScript(t), source, target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("credential removal failed: %v: %s", err, output)
	}
	result, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "openai-codex") ||
		!strings.Contains(string(result), "another-provider") {
		t.Fatalf("unexpected credential result: %s", result)
	}
}

func TestUninstallCredentialRemovalSignalsEmptyStore(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "auth.json")
	target := filepath.Join(dir, "auth.next.json")
	if err := os.WriteFile(
		source,
		[]byte(`{"openai-codex":{"type":"oauth","access":"secret","refresh":"secret"}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "-e", uninstallCredentialJavaScript(t), source, target)
	err = command.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Fatalf("expected empty-store exit code 3, got %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected replacement credential: %v", err)
	}
}

func TestFullCleanupRequiresOwnershipAndRemovesOnlyRecordedComponents(t *testing.T) {
	runner := &fakeRunner{output: "full_cleanup=removed\n"}
	if _, err := FullCleanup(runner); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"test -d \"$OWN\"",
		"[ -e \"$OWN/xovi\" ]",
		"[ -e \"$OWN/appload\" ]",
		"[ -e \"$OWN/runtime\" ]",
		"[ -e \"$OWN/oauth.sha256\" ]",
		"preserved_other_pi_credentials=1",
		"full_cleanup=removed",
	} {
		if !strings.Contains(runner.command, required) {
			t.Fatalf("full cleanup is missing %q", required)
		}
	}
}

func TestFullCleanupRequiresDeviceConfirmation(t *testing.T) {
	if _, err := FullCleanup(&fakeRunner{output: "done\n"}); err == nil {
		t.Fatal("missing full cleanup confirmation was accepted")
	}
	if _, err := FullCleanup(&fakeRunner{output: "failed\n", err: errors.New("exit 1")}); err == nil {
		t.Fatal("full cleanup command failure was accepted")
	}
}

func TestUninstallShellSyntax(t *testing.T) {
	for name, script := range map[string]string{
		"sign-out": SignOutChatGPTCommand,
		"safe":     UninstallCommand,
		"full":     FullCleanupCommand,
	} {
		command := exec.Command("sh", "-n")
		command.Stdin = strings.NewReader(script)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%s uninstall shell syntax: %v: %s", name, err, output)
		}
	}
}

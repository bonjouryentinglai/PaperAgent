// SPDX-License-Identifier: MIT
package main

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
)

type appRoundTripFunc func(*http.Request) (*http.Response, error)

func (function appRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestLinkedReleaseTagWhenPresent(t *testing.T) {
	if releaseTag == "" {
		t.Skip("ordinary build has no embedded release tag")
	}
	const expected = "https://github.com/bonjouryentinglai/PaperAgent/releases/download/v0.2.0-rc.1/paper-agent-manifest.json"
	if actual := releaseManifestURL(); actual != expected {
		t.Fatalf("linked release manifest URL = %q, want %q", actual, expected)
	}
}

func TestReleaseManifestURLUsesEmbeddedReleaseTag(t *testing.T) {
	previous := releaseTag
	t.Cleanup(func() { releaseTag = previous })
	releaseTag = "v0.2.0-rc.1"

	const expected = "https://github.com/bonjouryentinglai/PaperAgent/releases/download/v0.2.0-rc.1/paper-agent-manifest.json"
	if actual := releaseManifestURL(); actual != expected {
		t.Fatalf("release manifest URL = %q, want %q", actual, expected)
	}
}

func TestReleaseManifestURLRejectsUnsafeOrNonReleaseTags(t *testing.T) {
	previous := releaseTag
	t.Cleanup(func() { releaseTag = previous })
	for _, value := range []string{
		"",
		"codex/phase-2b-settings-uninstall",
		"v0.2.0/../../latest",
		"0.2.0",
	} {
		releaseTag = value
		if actual := releaseManifestURL(); actual != defaultReleaseManifestURL {
			t.Fatalf("tag %q selected manifest URL %q", value, actual)
		}
	}
}

func TestReleaseManifestEnvironmentOverrideTakesPriority(t *testing.T) {
	previous := releaseTag
	t.Cleanup(func() { releaseTag = previous })
	releaseTag = "v0.2.0-rc.1"
	t.Setenv("PAPER_AGENT_RELEASE_MANIFEST_URL", "http://127.0.0.1:8765/test.json")

	if actual := releaseManifestURL(); actual != "http://127.0.0.1:8765/test.json" {
		t.Fatalf("release manifest URL = %q", actual)
	}
}

func TestCheckReleaseUsesShortRequestDeadline(t *testing.T) {
	t.Setenv("PAPER_AGENT_RELEASE_MANIFEST_URL", "https://example.invalid/manifest.json")
	var remaining time.Duration
	app := NewApp()
	app.httpClient = &http.Client{Transport: appRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Fatal("release check request has no deadline")
		}
		remaining = time.Until(deadline)
		return nil, errors.New("stop after inspecting deadline")
	})}
	if _, err := app.CheckRelease(); err == nil {
		t.Fatal("release check unexpectedly succeeded")
	}
	if remaining <= 0 || remaining > releaseCheckTimeout+time.Second {
		t.Fatalf("release check deadline remaining = %s", remaining)
	}
}

func TestPauseForLoginCompletesInstallProgress(t *testing.T) {
	app := NewApp()
	status := preflight.Status{PaperAgent: true, SettingsApp: true, ActivationRequired: true}
	app.pauseForLogin(status, "Continue in Step 4.")
	state := app.GetOperationState()
	if state.Running || !state.Done || !state.NeedsLogin {
		t.Fatalf("unexpected paused operation state: %+v", state)
	}
	if state.Stage != "installed" || state.Percent != 100 {
		t.Fatalf("install did not complete before login: stage=%q percent=%d", state.Stage, state.Percent)
	}
}

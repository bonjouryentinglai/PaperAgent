// SPDX-License-Identifier: MIT
package main

import "testing"

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

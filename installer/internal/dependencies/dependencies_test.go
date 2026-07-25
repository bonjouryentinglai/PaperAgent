// SPDX-License-Identifier: MIT
package dependencies

import (
	"strings"
	"testing"
)

func TestPinnedDependenciesAreBoundedAndHTTPS(t *testing.T) {
	seen := make(map[string]bool)
	for _, dependency := range All() {
		if dependency.Name == "" || dependency.Filename == "" {
			t.Fatalf("incomplete dependency: %#v", dependency)
		}
		if seen[dependency.Filename] {
			t.Fatalf("duplicate dependency filename %q", dependency.Filename)
		}
		seen[dependency.Filename] = true
		if !strings.HasPrefix(dependency.Artifact.URL, "https://") {
			t.Fatalf("%s does not use HTTPS", dependency.Name)
		}
		if len(dependency.Artifact.SHA256) != 64 || dependency.Artifact.Bytes <= 0 {
			t.Fatalf("%s is not checksum/size pinned", dependency.Name)
		}
	}
}

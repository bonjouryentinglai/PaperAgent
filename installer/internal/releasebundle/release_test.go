// SPDX-License-Identifier: MIT
package releasebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestFetchValidatesAndResolvesBundle(t *testing.T) {
	payload := []byte("release")
	digest := sha256.Sum256(payload)
	body := `{
  "schema": 1,
  "version": "0.2.0",
  "supportedModels": ["chiappa"],
  "minimumFreeSpaceKB": 262144,
  "bundle": {
    "url": "paper-agent-release.tar.gz",
    "sha256": "` + hex.EncodeToString(digest[:]) + `",
    "bytes": 7
  }
}`
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://example.test/releases/manifest.json" {
			t.Fatalf("unexpected URL %s", request.URL)
		}
		return response(http.StatusOK, body), nil
	})}
	manifest, err := Fetch(context.Background(), client, "https://example.test/releases/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Bundle.URL != "https://example.test/releases/paper-agent-release.tar.gz" {
		t.Fatalf("unexpected bundle URL %q", manifest.Bundle.URL)
	}
}

func TestManifestRejectsUnsafeOrUnsupportedInput(t *testing.T) {
	valid := Manifest{
		Schema:          1,
		Version:         "0.2.0",
		SupportedModels: []string{"chiappa"},
		Bundle: Artifact{
			URL:    "https://example.test/bundle",
			SHA256: strings.Repeat("a", 64),
			Bytes:  10,
		},
	}
	tests := []struct {
		name     string
		url      string
		manifest Manifest
	}{
		{"HTTP manifest", "http://example.test/manifest.json", valid},
		{"wrong model", "https://example.test/manifest.json", func() Manifest {
			value := valid
			value.SupportedModels = []string{"ferrari"}
			return value
		}()},
		{"bad checksum", "https://example.test/manifest.json", func() Manifest {
			value := valid
			value.Bundle.SHA256 = "no"
			return value
		}()},
		{"oversize", "https://example.test/manifest.json", func() Manifest {
			value := valid
			value.Bundle.Bytes = MaxBundleBytes + 1
			return value
		}()},
	}
	for _, current := range tests {
		t.Run(current.name, func(t *testing.T) {
			if _, err := current.manifest.Validate(current.url); err == nil {
				t.Fatal("unsafe manifest was accepted")
			}
		})
	}
}

func TestDownloadVerifiesSizeAndChecksumBeforeActivation(t *testing.T) {
	payload := []byte("verified release payload")
	digest := sha256.Sum256(payload)
	artifact := Artifact{
		URL:    "https://example.test/release.tar.gz",
		SHA256: hex.EncodeToString(digest[:]),
		Bytes:  int64(len(payload)),
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})}
	directory := t.TempDir()
	path, err := Download(context.Background(), client, artifact, directory)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded %q", got)
	}

	artifact.SHA256 = strings.Repeat("0", 64)
	if _, err := Download(context.Background(), client, artifact, t.TempDir()); err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

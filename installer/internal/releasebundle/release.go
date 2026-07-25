// SPDX-License-Identifier: MIT
package releasebundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	MaxManifestBytes = 1 << 20
	MaxBundleBytes   = 2 << 30
)

var (
	versionPattern  = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]{0,63}$`)
	checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Artifact struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Manifest struct {
	Schema             int      `json:"schema"`
	Version            string   `json:"version"`
	SupportedModels    []string `json:"supportedModels"`
	MinimumFreeSpaceKB int64    `json:"minimumFreeSpaceKB"`
	Bundle             Artifact `json:"bundle"`
}

func secureURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return parsed, nil
	}
	// Tests and maintainer-only local previews may use loopback HTTP. Published
	// manifests and bundles must use HTTPS.
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "127.0.0.1" || host == "::1" || host == "localhost") {
		return parsed, nil
	}
	return nil, fmt.Errorf("URL must use HTTPS")
}

func (manifest Manifest) Validate(manifestURL string) (Manifest, error) {
	base, err := secureURL(manifestURL)
	if err != nil {
		return Manifest{}, fmt.Errorf("manifest URL: %w", err)
	}
	if manifest.Schema != 1 {
		return Manifest{}, fmt.Errorf("unsupported release manifest schema %d", manifest.Schema)
	}
	if !versionPattern.MatchString(manifest.Version) {
		return Manifest{}, fmt.Errorf("invalid release version")
	}
	if manifest.MinimumFreeSpaceKB < 0 {
		return Manifest{}, fmt.Errorf("minimum free space cannot be negative")
	}
	hasMove := false
	for _, model := range manifest.SupportedModels {
		if strings.EqualFold(strings.TrimSpace(model), "chiappa") {
			hasMove = true
		}
	}
	if !hasMove {
		return Manifest{}, fmt.Errorf("release does not declare Paper Pro Move support")
	}
	if manifest.Bundle.Bytes <= 0 || manifest.Bundle.Bytes > MaxBundleBytes {
		return Manifest{}, fmt.Errorf("invalid release bundle size")
	}
	manifest.Bundle.SHA256 = strings.ToLower(strings.TrimSpace(manifest.Bundle.SHA256))
	if !checksumPattern.MatchString(manifest.Bundle.SHA256) {
		return Manifest{}, fmt.Errorf("invalid release bundle SHA-256")
	}
	bundleURL, err := url.Parse(strings.TrimSpace(manifest.Bundle.URL))
	if err != nil {
		return Manifest{}, fmt.Errorf("release bundle URL: %w", err)
	}
	bundleURL = base.ResolveReference(bundleURL)
	if _, err := secureURL(bundleURL.String()); err != nil {
		return Manifest{}, fmt.Errorf("release bundle URL: %w", err)
	}
	manifest.Bundle.URL = bundleURL.String()
	return manifest, nil
}

func Fetch(ctx context.Context, client *http.Client, manifestURL string) (Manifest, error) {
	if client == nil {
		return Manifest{}, fmt.Errorf("HTTP client is required")
	}
	if _, err := secureURL(manifestURL); err != nil {
		return Manifest{}, fmt.Errorf("manifest URL: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return Manifest{}, fmt.Errorf("download release manifest: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("download release manifest: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxManifestBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("download release manifest: %w", err)
	}
	if len(body) > MaxManifestBytes {
		return Manifest{}, fmt.Errorf("release manifest is too large")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode release manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Manifest{}, fmt.Errorf("decode release manifest: trailing data")
	}
	return manifest.Validate(manifestURL)
}

func Download(ctx context.Context, client *http.Client, artifact Artifact, directory string) (string, error) {
	return DownloadAs(ctx, client, artifact, directory, "paper-agent-release.tar.gz")
}

func DownloadAs(
	ctx context.Context,
	client *http.Client,
	artifact Artifact,
	directory string,
	filename string,
) (string, error) {
	if client == nil {
		return "", fmt.Errorf("HTTP client is required")
	}
	if filename == "" || filename != filepath.Base(filename) || filename == "." {
		return "", fmt.Errorf("invalid release filename")
	}
	if _, err := secureURL(artifact.URL); err != nil {
		return "", fmt.Errorf("release bundle URL: %w", err)
	}
	checksum := strings.ToLower(strings.TrimSpace(artifact.SHA256))
	if !checksumPattern.MatchString(checksum) {
		return "", fmt.Errorf("invalid release bundle SHA-256")
	}
	if artifact.Bytes <= 0 || artifact.Bytes > MaxBundleBytes {
		return "", fmt.Errorf("invalid release bundle size")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create release staging directory: %w", err)
	}
	target := filepath.Join(directory, filename)
	partial, err := os.CreateTemp(directory, ".paper-agent-release-*.partial")
	if err != nil {
		return "", fmt.Errorf("create release staging file: %w", err)
	}
	partialName := partial.Name()
	keep := false
	defer func() {
		_ = partial.Close()
		if !keep {
			_ = os.Remove(partialName)
		}
	}()
	if err := partial.Chmod(0o600); err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("download release bundle: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download release bundle: HTTP %d", response.StatusCode)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(partial, hash), io.LimitReader(response.Body, artifact.Bytes+1))
	if err != nil {
		return "", fmt.Errorf("download release bundle: %w", err)
	}
	if written != artifact.Bytes {
		return "", fmt.Errorf("release bundle size mismatch: expected %d bytes, received %d", artifact.Bytes, written)
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != checksum {
		return "", fmt.Errorf("release bundle checksum mismatch")
	}
	if err := partial.Sync(); err != nil {
		return "", err
	}
	if err := partial.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(partialName, target); err != nil {
		return "", fmt.Errorf("activate verified release bundle: %w", err)
	}
	keep = true
	return target, nil
}

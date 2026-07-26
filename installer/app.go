// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/maintenance"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/oauth"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/releasebundle"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/setup"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	httpClient  *http.Client
	login       *oauth.Manager
	operation   OperationState
	operationMu sync.Mutex
}

type Candidate struct {
	Address       string `json:"address"`
	Banner        string `json:"banner"`
	DeveloperMode bool   `json:"developerMode"`
	USB           bool   `json:"usb"`
}

type OperationResult struct {
	Status  preflight.Status `json:"status"`
	Summary string           `json:"summary"`
}

type ReleaseSummary struct {
	Version            string `json:"version"`
	MinimumFreeSpaceKB int64  `json:"minimumFreeSpaceKB"`
	BundleBytes        int64  `json:"bundleBytes"`
}

type OperationState struct {
	Kind       string           `json:"kind"`
	Running    bool             `json:"running"`
	Done       bool             `json:"done"`
	NeedsLogin bool             `json:"needsLogin"`
	Stage      string           `json:"stage"`
	Message    string           `json:"message"`
	Error      string           `json:"error"`
	Percent    int              `json:"percent"`
	HasStatus  bool             `json:"hasStatus"`
	Status     preflight.Status `json:"status"`
}

const defaultReleaseManifestURL = "https://github.com/bonjouryentinglai/PaperAgent/releases/latest/download/paper-agent-manifest.json"

func NewApp() *App {
	return &App{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		login:      oauth.NewManager(),
		operation: OperationState{
			Message: "No install or maintenance operation is running.",
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(context.Context) {
	a.login.Cancel()
}

func (a *App) OpenLoginURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" {
		return fmt.Errorf("invalid ChatGPT login URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "chatgpt.com" && host != "auth.openai.com" &&
		!strings.HasSuffix(host, ".openai.com") {
		return fmt.Errorf("unexpected ChatGPT login host")
	}
	if a.ctx == nil {
		return fmt.Errorf("desktop application is not ready")
	}
	runtime.BrowserOpenURL(a.ctx, parsed.String())
	return nil
}

// Discover probes only the Move's documented USB address. Wi-Fi addresses may
// still be entered manually, but are never scanned or selected automatically.
// The probe is read-only and does not authenticate or change the device.
func (a *App) Discover() []Candidate {
	probe, err := device.ProbeAddr(device.DefaultUSBAddr, 650*time.Millisecond)
	if err != nil || !probe.IsPaperPro() {
		probe, err = device.ProbeAddr(device.DefaultUSBAddr, 2*time.Second)
	}
	if err != nil || !probe.IsPaperPro() {
		return nil
	}
	return []Candidate{{
		Address:       probe.Addr,
		Banner:        probe.Banner,
		DeveloperMode: true,
		USB:           true,
	}}
}

// Inspect authenticates only long enough to run an allowlisted, read-only
// preflight. Passwords are held in memory for this call and are never logged.
func (a *App) Inspect(host, password string) (preflight.Status, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = device.DefaultUSBAddr
	}
	connection, err := device.Connect(host, strings.TrimSpace(password))
	if err != nil {
		return preflight.Status{}, fmt.Errorf("connect to Move: %w", err)
	}
	defer connection.Close()
	return preflight.Inspect(connection, host)
}

// CheckRelease downloads only the small release manifest. The actual bundle is
// not trusted until releasebundle.Download verifies its declared byte length
// and SHA-256 checksum.
func (a *App) CheckRelease() (ReleaseSummary, error) {
	manifestURL := releaseManifestURL()
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	manifest, err := releasebundle.Fetch(ctx, a.httpClient, manifestURL)
	if err != nil {
		return ReleaseSummary{}, err
	}
	return ReleaseSummary{
		Version:            manifest.Version,
		MinimumFreeSpaceKB: manifest.MinimumFreeSpaceKB,
		BundleBytes:        manifest.Bundle.Bytes,
	}, nil
}

func releaseManifestURL() string {
	manifestURL := strings.TrimSpace(os.Getenv("PAPER_AGENT_RELEASE_MANIFEST_URL"))
	if manifestURL == "" {
		return defaultReleaseManifestURL
	}
	return manifestURL
}

func (a *App) setOperation(update func(*OperationState)) {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	update(&a.operation)
}

func (a *App) GetOperationState() OperationState {
	a.operationMu.Lock()
	defer a.operationMu.Unlock()
	return a.operation
}

func (a *App) startOperation(kind, host, password string, acknowledged bool) error {
	if !acknowledged {
		return fmt.Errorf("confirm the device changes first")
	}
	switch kind {
	case "install", "update", "repair":
	default:
		return fmt.Errorf("unsupported operation")
	}
	if a.login.State().Running {
		return fmt.Errorf("finish or cancel ChatGPT login before changing the installation")
	}
	a.operationMu.Lock()
	if a.operation.Running {
		a.operationMu.Unlock()
		return fmt.Errorf("another install or maintenance operation is already running")
	}
	a.operation = OperationState{
		Kind:    kind,
		Running: true,
		Stage:   "connect",
		Message: "Connecting to the Move…",
		Percent: 2,
	}
	a.operationMu.Unlock()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = device.DefaultUSBAddr
	}
	go a.runOperation(ctx, kind, host, strings.TrimSpace(password))
	return nil
}

func (a *App) StartInstall(host, password string, acknowledged bool) error {
	return a.startOperation("install", host, password, acknowledged)
}

func (a *App) StartUpdate(host, password string, acknowledged bool) error {
	return a.startOperation("update", host, password, acknowledged)
}

func (a *App) StartRepair(host, password string, acknowledged bool) error {
	return a.startOperation("repair", host, password, acknowledged)
}

func (a *App) finishOperation(message string, status *preflight.Status, err error) {
	a.setOperation(func(state *OperationState) {
		state.Running = false
		state.Done = true
		state.Percent = 100
		state.Message = message
		if status != nil {
			state.Status = *status
			state.HasStatus = true
		}
		if err != nil {
			state.Error = err.Error()
		}
	})
}

func (a *App) operationProgress(stage, message string, percent int) {
	a.setOperation(func(state *OperationState) {
		state.Stage = stage
		state.Message = message
		state.Percent = percent
	})
}

func (a *App) runOperation(ctx context.Context, kind, host, password string) {
	connection, err := device.Connect(host, password)
	if err != nil {
		a.finishOperation("Could not connect to the Move.", nil, err)
		return
	}
	status, err := preflight.Inspect(connection, host)
	connection.Close()
	if err != nil {
		a.finishOperation("Device preflight failed.", nil, err)
		return
	}
	if !status.SupportedModel {
		a.finishOperation("This device is not a supported Paper Pro Move.", &status, fmt.Errorf("unsupported model: %s", status.Model))
		return
	}
	if kind == "install" && status.PaperAgent {
		a.finishOperation("Paper Agent is already installed; use Update or Repair.", &status, fmt.Errorf("Paper Agent is already installed"))
		return
	}
	if kind == "update" && !status.PaperAgent {
		a.finishOperation("Paper Agent is not installed; use Install.", &status, fmt.Errorf("Paper Agent is not installed"))
		return
	}
	if kind == "repair" && !status.PaperAgent {
		a.finishOperation("Paper Agent is not installed; use Install.", &status, fmt.Errorf("Paper Agent is not installed"))
		return
	}
	a.operationProgress("release", "Checking the published Paper Agent release…", 4)
	manifest, err := releasebundle.Fetch(ctx, a.httpClient, releaseManifestURL())
	if err != nil {
		a.finishOperation("Could not load a Paper Agent release; no device changes were made.", &status, err)
		return
	}
	if status.FreeSpaceKB < manifest.MinimumFreeSpaceKB {
		a.finishOperation("The Move does not have enough free storage.", &status, fmt.Errorf(
			"release requires %d KiB free; device reports %d KiB",
			manifest.MinimumFreeSpaceKB,
			status.FreeSpaceKB,
		))
		return
	}

	a.operationProgress("prerequisites", "Checking and installing verified prerequisites…", 5)
	status, err = setup.Ensure(
		ctx,
		a.httpClient,
		host,
		password,
		status,
		kind == "repair",
		a.operationProgress,
	)
	if err != nil {
		a.finishOperation("Prerequisite setup failed.", &status, err)
		return
	}
	if !status.ChatGPTLoggedIn {
		a.setOperation(func(state *OperationState) {
			state.Running = false
			state.Done = true
			state.NeedsLogin = true
			state.Stage = "login"
			state.Message = "Prerequisites are ready. Sign in to ChatGPT, then run this operation again."
			state.Percent = 100
			state.Status = status
			state.HasStatus = true
		})
		return
	}

	stage, err := os.MkdirTemp("", "paper-agent-release-*")
	if err != nil {
		a.finishOperation("Could not create release staging.", &status, err)
		return
	}
	defer os.RemoveAll(stage)

	a.operationProgress("release", "Downloading and verifying the Paper Agent release…", 91)
	bundle, err := releasebundle.Download(ctx, a.httpClient, manifest.Bundle, stage)
	if err != nil {
		a.finishOperation("Release verification failed.", &status, err)
		return
	}
	connection, err = device.Connect(host, password)
	if err != nil {
		a.finishOperation("Could not reconnect to the Move.", &status, err)
		return
	}
	a.operationProgress("install", "Applying the release with backup and rollback protection…", 95)
	_, deployErr := setup.Deploy(connection, bundle)
	connection.Close()

	var verified preflight.Status
	for attempt := 0; attempt < 30; attempt++ {
		connection, err = device.Connect(host, password)
		if err == nil {
			verified, err = preflight.Inspect(connection, host)
			connection.Close()
			if err == nil {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		if deployErr != nil {
			err = fmt.Errorf("%v; verification: %w", deployErr, err)
		}
		a.finishOperation("Could not verify the installation.", &status, err)
		return
	}
	if !verified.PaperAgent || !verified.ServiceActive || !verified.SettingsApp ||
		verified.InstalledVersion != manifest.Version {
		if deployErr == nil {
			deployErr = fmt.Errorf("installed components or version did not pass verification")
		}
		a.finishOperation("Paper Agent did not pass post-install verification.", &verified, deployErr)
		return
	}
	a.finishOperation(
		fmt.Sprintf("Paper Agent %s is installed and running.", manifest.Version),
		&verified,
		nil,
	)
}

func (a *App) StartLogin(host, password string) error {
	if a.GetOperationState().Running {
		return fmt.Errorf("wait for the current install or maintenance operation to finish")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.login.Start(ctx, strings.TrimSpace(host), strings.TrimSpace(password))
}

func (a *App) GetLoginState() oauth.State {
	return a.login.State()
}

func (a *App) CancelLogin() {
	a.login.Cancel()
}

func owns(status preflight.Status, component string) bool {
	for _, current := range status.InstallerOwned {
		if current == component {
			return true
		}
	}
	return false
}

// Uninstall defaults to removing Paper Agent only. Full cleanup is available
// only when installation-time ownership markers prove which shared components
// Paper Agent Installer originally created.
func (a *App) Uninstall(host, password string, acknowledged, full bool) (OperationResult, error) {
	if !acknowledged {
		return OperationResult{}, fmt.Errorf("confirm the uninstall first")
	}
	if a.GetOperationState().Running || a.login.State().Running {
		return OperationResult{}, fmt.Errorf("finish the current operation before uninstalling")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = device.DefaultUSBAddr
	}
	connection, err := device.Connect(host, strings.TrimSpace(password))
	if err != nil {
		return OperationResult{}, fmt.Errorf("connect to Move: %w", err)
	}
	defer connection.Close()

	before, err := preflight.Inspect(connection, host)
	if err != nil {
		return OperationResult{}, err
	}
	if !before.SupportedModel {
		return OperationResult{}, fmt.Errorf("unsupported device: Paper Agent currently supports Paper Pro Move (chiappa) only")
	}
	if full && len(before.InstallerOwned) == 0 {
		return OperationResult{}, fmt.Errorf("full cleanup is unavailable because no installer ownership record exists")
	}
	if !before.PaperAgent && !full {
		return OperationResult{Status: before, Summary: "Paper Agent is not installed."}, nil
	}
	if before.PaperAgent {
		if _, err := maintenance.Uninstall(connection); err != nil {
			return OperationResult{}, err
		}
	}
	cleanupOutput := ""
	if full {
		cleanupOutput, err = maintenance.FullCleanup(connection)
		if err != nil {
			return OperationResult{}, err
		}
	}
	after, err := preflight.Inspect(connection, host)
	if err != nil {
		return OperationResult{}, fmt.Errorf("verify uninstall: %w", err)
	}
	if after.PaperAgent || after.ServiceActive || after.SettingsApp {
		return OperationResult{}, fmt.Errorf("uninstall verification failed: Paper Agent components remain")
	}
	if full {
		if len(after.InstallerOwned) != 0 {
			return OperationResult{}, fmt.Errorf("full cleanup verification failed: ownership records remain")
		}
		if owns(before, "xovi") && after.XOVIInstalled {
			return OperationResult{}, fmt.Errorf("full cleanup verification failed: installer-managed XOVI remains")
		}
		if owns(before, "appload") && after.AppLoadInstalled {
			return OperationResult{}, fmt.Errorf("full cleanup verification failed: installer-managed AppLoad remains")
		}
		if owns(before, "runtime") && (after.NodeInstalled || after.PiInstalled) {
			return OperationResult{}, fmt.Errorf("full cleanup verification failed: installer-managed runtime remains")
		}
		if owns(before, "pi-packages") && !owns(before, "runtime") && after.PiInstalled {
			return OperationResult{}, fmt.Errorf("full cleanup verification failed: installer-managed Pi packages remain")
		}
		summary := "Paper Agent and all components proven to be installed by Paper Agent Installer were removed."
		if strings.Contains(cleanupOutput, "preserved_changed_oauth=1") {
			summary += " ChatGPT sign-in was preserved because it changed after the installer recorded it."
		}
		return OperationResult{Status: after, Summary: summary}, nil
	}
	return OperationResult{
		Status:  after,
		Summary: "Paper Agent was removed. ChatGPT sign-in, XOVI/AppLoad, settings, runtime, and backups were preserved.",
	}, nil
}

func (a *App) ImplementationState() map[string]bool {
	return map[string]bool{
		"discovery": true,
		"preflight": true,
		"release":   true,
		"install":   true,
		"update":    true,
		"repair":    true,
		"login":     true,
		"uninstall": true,
	}
}

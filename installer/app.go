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
	RuntimeBytes       int64  `json:"runtimeBytes"`
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

const (
	defaultReleaseManifestURL = "https://github.com/bonjouryentinglai/PaperAgent/releases/latest/download/paper-agent-manifest.json"
	taggedReleaseManifestURL  = "https://github.com/bonjouryentinglai/PaperAgent/releases/download/%s/paper-agent-manifest.json"
)

// releaseTag is set only for tagged release builds through Go linker flags.
// Branch, pull-request, and local builds leave it empty and use latest.
var releaseTag string

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
	var runtimeBytes int64
	if manifest.Runtime != nil {
		runtimeBytes = manifest.Runtime.Bytes
	}
	return ReleaseSummary{
		Version:            manifest.Version,
		MinimumFreeSpaceKB: manifest.MinimumFreeSpaceKB,
		BundleBytes:        manifest.Bundle.Bytes,
		RuntimeBytes:       runtimeBytes,
	}, nil
}

func releaseManifestURL() string {
	manifestURL := strings.TrimSpace(os.Getenv("PAPER_AGENT_RELEASE_MANIFEST_URL"))
	if manifestURL != "" {
		return manifestURL
	}
	tag := strings.TrimSpace(releaseTag)
	if validReleaseTag(tag) {
		return fmt.Sprintf(taggedReleaseManifestURL, tag)
	}
	return defaultReleaseManifestURL
}

func validReleaseTag(tag string) bool {
	if len(tag) < 2 || tag[0] != 'v' {
		return false
	}
	for index := 1; index < len(tag); index++ {
		current := tag[index]
		if (current >= 'a' && current <= 'z') ||
			(current >= 'A' && current <= 'Z') ||
			(current >= '0' && current <= '9') ||
			current == '.' || current == '_' || current == '+' || current == '-' {
			continue
		}
		return false
	}
	return true
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

func (a *App) pauseForLogin(status preflight.Status, message string) {
	a.setOperation(func(state *OperationState) {
		state.Running = false
		state.Done = false
		state.NeedsLogin = true
		state.Stage = "login"
		state.Message = message
		state.Percent = 98
		state.Status = status
		state.HasStatus = true
	})
}

func inspectWithRetry(host, password string, attempts int) (preflight.Status, error) {
	var status preflight.Status
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		connection, err := device.Connect(host, password)
		if err == nil {
			status, err = preflight.Inspect(connection, host)
			connection.Close()
			if err == nil {
				return status, nil
			}
		}
		last = err
		time.Sleep(2 * time.Second)
	}
	return status, last
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
	if kind == "install" && status.PaperAgent && !status.ActivationRequired {
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
		manifest.Runtime,
		kind == "repair",
		a.operationProgress,
	)
	if err != nil {
		a.finishOperation("Prerequisite setup failed.", &status, err)
		return
	}
	if !status.ChatGPTLoggedIn && kind != "install" {
		a.pauseForLogin(status, "Paper Agent is installed, but ChatGPT is signed out. Sign in to continue this maintenance operation.")
		return
	}
	if !status.ChatGPTLoggedIn && status.ActivationRequired && status.SettingsApp &&
		status.InstalledVersion == manifest.Version {
		a.pauseForLogin(status, "Paper Agent and Settings are installed. Sign in to ChatGPT; activation will continue automatically.")
		return
	}

	stage, err := os.MkdirTemp("", "paper-agent-release-*")
	if err != nil {
		a.finishOperation("Could not create release staging.", &status, err)
		return
	}
	defer os.RemoveAll(stage)

	a.operationProgress("release", "Downloading and verifying the Paper Agent release…", 95)
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
	if !status.ChatGPTLoggedIn {
		a.operationProgress("install", "Installing Paper Agent and Settings before ChatGPT sign-in…", 97)
		_, deployErr := setup.DeployStaged(connection, bundle)
		connection.Close()
		verified, verifyErr := inspectWithRetry(host, password, 30)
		if verifyErr != nil {
			if deployErr != nil {
				verifyErr = fmt.Errorf("%v; verification: %w", deployErr, verifyErr)
			}
			a.finishOperation("Could not verify the staged installation.", &status, verifyErr)
			return
		}
		if !verified.PaperAgent || !verified.SettingsApp || !verified.ActivationRequired ||
			verified.InstalledVersion != manifest.Version {
			if deployErr == nil {
				deployErr = fmt.Errorf("staged components or version did not pass verification")
			}
			a.finishOperation("Paper Agent and Settings did not pass staged verification.", &verified, deployErr)
			return
		}
		a.pauseForLogin(verified, "Paper Agent and Settings are installed. Sign in to ChatGPT; activation will continue automatically.")
		return
	}

	a.operationProgress("activate", "Activating Paper Agent with backup and rollback protection…", 99)
	_, deployErr := setup.Deploy(connection, bundle)
	connection.Close()

	verified, err := inspectWithRetry(host, password, 30)
	if err != nil {
		if deployErr != nil {
			err = fmt.Errorf("%v; verification: %w", deployErr, err)
		}
		a.finishOperation("Could not verify the installation.", &status, err)
		return
	}
	if !verified.PaperAgent || verified.ActivationRequired || !verified.ServiceActive || !verified.SettingsApp ||
		verified.InstalledVersion != manifest.Version {
		if deployErr == nil {
			deployErr = fmt.Errorf("installed components or version did not pass verification")
		}
		a.finishOperation("Paper Agent did not pass post-install verification.", &verified, deployErr)
		return
	}
	a.finishOperation(
		fmt.Sprintf(
			"Paper Agent %s is installed and running. If AppLoad is not visible, quickly press the Move power button three times to start XOVI.",
			manifest.Version,
		),
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
		if strings.Contains(cleanupOutput, "preserved_other_pi_credentials=1") {
			summary += " Other Pi provider credentials were preserved."
		}
		return OperationResult{Status: after, Summary: summary}, nil
	}
	return OperationResult{
		Status:  after,
		Summary: "Paper Agent was removed and ChatGPT was signed out. XOVI/AppLoad, settings, runtime, and backups were preserved.",
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

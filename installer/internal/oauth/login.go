// SPDX-License-Identifier: MIT
package oauth

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
)

var (
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	urlPattern  = regexp.MustCompile(`https://[^\s]+`)
	codePattern = regexp.MustCompile(`(?i)Enter code:\s*([A-Z0-9-]{4,})`)
)

type State struct {
	Running  bool   `json:"running"`
	Done     bool   `json:"done"`
	SignedIn bool   `json:"signedIn"`
	URL      string `json:"url"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Error    string `json:"error"`
}

type Manager struct {
	mu         sync.Mutex
	state      State
	transcript string
	cancel     context.CancelFunc
}

func NewManager() *Manager {
	return &Manager{state: State{Message: "ChatGPT login has not started."}}
}

func (manager *Manager) State() State {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.state
}

func (manager *Manager) Start(parent context.Context, host, password string) error {
	manager.mu.Lock()
	if manager.state.Running {
		manager.mu.Unlock()
		return fmt.Errorf("a ChatGPT login is already running")
	}
	ctx, cancel := context.WithCancel(parent)
	manager.cancel = cancel
	manager.transcript = ""
	manager.state = State{
		Running: true,
		Message: "Connecting to Pi on the Move…",
	}
	manager.mu.Unlock()

	go manager.run(ctx, strings.TrimSpace(host), strings.TrimSpace(password))
	return nil
}

func (manager *Manager) Cancel() {
	manager.mu.Lock()
	cancel := manager.cancel
	if manager.state.Running {
		manager.state.Message = "Cancelling ChatGPT login…"
	}
	manager.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (manager *Manager) updateOutput(chunk string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	clean := ansiPattern.ReplaceAllString(chunk, "")
	manager.transcript += clean
	if len(manager.transcript) > 16384 {
		manager.transcript = manager.transcript[len(manager.transcript)-16384:]
	}
	if match := urlPattern.FindString(manager.transcript); match != "" {
		manager.state.URL = strings.TrimRight(match, ".,;)")
	}
	if match := codePattern.FindStringSubmatch(manager.transcript); len(match) == 2 {
		manager.state.Code = match[1]
	}
	if manager.state.URL != "" && manager.state.Code != "" {
		manager.state.Message = "Open the URL, enter the code, and approve ChatGPT access."
	} else {
		manager.state.Message = "Waiting for Pi to provide the ChatGPT device code…"
	}
}

func (manager *Manager) finish(signedIn bool, message string, err error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.cancel = nil
	manager.state.Running = false
	manager.state.Done = true
	manager.state.SignedIn = signedIn
	manager.state.Message = message
	if err != nil {
		manager.state.Error = err.Error()
	}
}

func (manager *Manager) run(ctx context.Context, host, password string) {
	if host == "" {
		host = device.DefaultUSBAddr
	}
	connection, err := device.Connect(host, password)
	if err != nil {
		manager.finish(false, "Could not connect to the Move.", err)
		return
	}
	defer connection.Close()

	if _, err := connection.Run("test -x /home/root/node/bin/pi-ai"); err != nil {
		manager.finish(false, "Install the Paper Agent runtime before signing in.", fmt.Errorf("Pi is not installed on the Move"))
		return
	}
	const command = `set -eu
umask 077
AUTH_DIR=/home/root/.pi/agent
mkdir -p "$AUTH_DIR"
chmod 0700 /home/root/.pi "$AUTH_DIR" 2>/dev/null || true
cd "$AUTH_DIR"
HOME=/home/root PATH="/home/root/node/bin:$PATH" \
  /home/root/node/bin/pi-ai login openai-codex
chmod 0600 "$AUTH_DIR/auth.json"
`
	_, err = connection.RunStreaming(
		ctx,
		command,
		strings.NewReader("2\n"),
		manager.updateOutput,
	)
	if err != nil {
		if ctx.Err() != nil {
			manager.finish(false, "ChatGPT login was cancelled.", ctx.Err())
			return
		}
		manager.finish(false, "ChatGPT login failed.", err)
		return
	}

	const verify = `/home/root/node/bin/node -e '
const fs=require("fs");
const value=JSON.parse(fs.readFileSync("/home/root/.pi/agent/auth.json","utf8"));
const auth=value["openai-codex"];
process.exit(auth && auth.type === "oauth" && auth.access && auth.refresh ? 0 : 1);
' >/dev/null 2>&1`
	if _, err := connection.Run(verify); err != nil {
		manager.finish(false, "Pi finished, but no valid ChatGPT credential was found.", err)
		return
	}
	const markOwnership = `set -eu
AUTH=/home/root/.pi/agent/auth.json
OWN=/home/root/paper-agent/installer-owned
mkdir -p "$OWN"
sha256sum "$AUTH" | awk '{print $1}' >"$OWN/oauth.sha256"
chmod 0600 "$OWN/oauth.sha256"
`
	if _, err := connection.Run(markOwnership); err != nil {
		manager.finish(
			true,
			"ChatGPT sign-in is ready. Installer ownership could not be recorded, but Paper Agent uninstall will still sign out.",
			nil,
		)
		return
	}
	manager.finish(true, "ChatGPT sign-in is ready on the Move.", nil)
}

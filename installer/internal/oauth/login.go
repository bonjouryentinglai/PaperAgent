// SPDX-License-Identifier: MIT
package oauth

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/maintenance"
)

var (
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	urlPattern  = regexp.MustCompile(`https://[^\s]+`)
	codePattern = regexp.MustCompile(`(?i)Enter code:\s*([A-Z0-9-]{4,})`)
	loginSerial atomic.Uint64
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
	loginID := fmt.Sprintf("%x-%x", time.Now().UnixNano(), loginSerial.Add(1))
	pidFile := "/run/paper-agent-installer-login-" + loginID + ".pid"
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
	command := loginCommand(pidFile)
	_, err = connection.RunStreaming(
		ctx,
		command,
		strings.NewReader("2\n"),
		manager.updateOutput,
	)
	if err != nil {
		if ctx.Err() != nil {
			if cleanupErr := cleanupCancelledLogin(host, password, pidFile); cleanupErr != nil {
				manager.finish(false, "ChatGPT login was cancelled, but cleanup needs attention.", cleanupErr)
				return
			}
			manager.finish(false, "ChatGPT login was cancelled.", nil)
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

func loginCommand(pidFile string) string {
	return `set -eu
umask 077
AUTH_DIR=/home/root/.pi/agent
PIDFILE=` + pidFile + `
mkdir -p "$AUTH_DIR"
chmod 0700 /home/root/.pi "$AUTH_DIR" 2>/dev/null || true
cd "$AUTH_DIR"
if sh -c '
  set -eu
  PIDFILE=$1
  printf "%s\n" "$$" >"$PIDFILE"
  chmod 0600 "$PIDFILE"
  exec env HOME=/home/root PATH="/home/root/node/bin:$PATH" \
    /home/root/node/bin/pi-ai login openai-codex
' paper-agent-login "$PIDFILE"; then
  login_status=0
else
  login_status=$?
fi
rm -f "$PIDFILE"
[ "$login_status" -eq 0 ] || exit "$login_status"
chmod 0600 "$AUTH_DIR/auth.json"
`
}

func cancelLoginCommand(pidFile string) string {
	return `set -eu
PIDFILE=` + pidFile + `
if [ -f "$PIDFILE" ]; then
  pid=$(cat "$PIDFILE")
  case "$pid" in
    ""|*[!0-9]*)
      echo "Invalid Paper Agent login PID" >&2
      exit 1
      ;;
  esac
  if [ -r "/proc/$pid/cmdline" ]; then
    command=$(tr '\000' ' ' <"/proc/$pid/cmdline")
    case "$command" in
      "node /home/root/node/bin/pi-ai login openai-codex "*|\
      "/home/root/node/bin/node /home/root/node/bin/pi-ai login openai-codex "*)
        kill -TERM "$pid" 2>/dev/null || true
        attempt=0
        while kill -0 "$pid" 2>/dev/null && [ "$attempt" -lt 2 ]; do
          sleep 1
          attempt=$((attempt + 1))
        done
        if kill -0 "$pid" 2>/dev/null; then
          kill -KILL "$pid"
        fi
        ;;
      *)
        echo "Refusing to stop an unrelated process" >&2
        exit 1
        ;;
    esac
  fi
  rm -f "$PIDFILE"
fi
echo "login_process=stopped"
`
}

func cleanupCancelledLogin(host, password, pidFile string) error {
	connection, err := device.Connect(host, password)
	if err != nil {
		return fmt.Errorf("reconnect to clean up cancelled login: %w", err)
	}
	defer connection.Close()
	output, err := connection.Run(cancelLoginCommand(pidFile))
	if err != nil {
		return fmt.Errorf("stop cancelled login: %w: %s", err, strings.TrimSpace(output))
	}
	if !strings.Contains(output, "login_process=stopped") {
		return fmt.Errorf("stop cancelled login: device did not confirm cleanup")
	}
	if _, err := maintenance.SignOutChatGPT(connection); err != nil {
		return err
	}
	return nil
}

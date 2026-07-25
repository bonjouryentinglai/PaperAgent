// SPDX-License-Identifier: MIT
//
// The XOVI/AppLoad setup sequence is adapted from Maxime Rivest's remagic
// installer. Paper Agent keeps the same pure-Go SSH boundary, but downloads
// every third-party input on the desktop and verifies its pinned checksum
// before sending it to the Move.
package setup

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/dependencies"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/releasebundle"
)

const (
	nodeVersion = "22.22.3"
	piVersion   = "0.80.7"
)

type Reporter func(stage, message string, percent int)

type prerequisitePlan struct {
	xoviArchive  bool
	appLoad      bool
	nativeBridge bool
	qmlIndex     bool
}

func planPrerequisites(status preflight.Status, forceRepair bool) prerequisitePlan {
	return prerequisitePlan{
		// XOVI and AppLoad are shared with other reMarkable tools. Repair
		// refreshes Paper Agent's own bridge and index, but never overwrites a
		// detected shared installation just because Repair was selected.
		xoviArchive:  !status.XOVIInstalled || !status.NativeDeps,
		appLoad:      !status.AppLoadInstalled,
		nativeBridge: forceRepair || !status.NativeDeps,
		qmlIndex:     forceRepair || !status.QMLIndex,
	}
}

func report(callback Reporter, stage, message string, percent int) {
	if callback != nil {
		callback(stage, message, percent)
	}
}

func tail(value string, length int) string {
	value = strings.TrimSpace(value)
	if len(value) <= length {
		return value
	}
	return "…" + value[len(value)-length:]
}

func runChecked(connection *device.Device, description, command string) error {
	output, err := connection.Run(command)
	if err != nil {
		return fmt.Errorf("%s: %w: %s", description, err, tail(output, 500))
	}
	return nil
}

func reconnect(host, password string, attempts int) (*device.Device, error) {
	var last error
	for count := 0; count < attempts; count++ {
		connection, err := device.Connect(host, password)
		if err == nil {
			return connection, nil
		}
		last = err
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("reconnect to Move: %w", last)
}

func fetchDependency(
	ctx context.Context,
	client *http.Client,
	dependency dependencies.Dependency,
	directory string,
) (string, error) {
	return releasebundle.DownloadAs(
		ctx,
		client,
		dependency.Artifact,
		directory,
		dependency.Filename,
	)
}

// Ensure installs only the prerequisites missing from status. It is safe to
// rerun for Repair: existing Paper Agent data and OAuth credentials are not
// touched.
func Ensure(
	ctx context.Context,
	client *http.Client,
	host string,
	password string,
	status preflight.Status,
	forceRepair bool,
	callback Reporter,
) (preflight.Status, error) {
	if !status.SupportedModel {
		return status, fmt.Errorf("unsupported device: Paper Agent currently supports Paper Pro Move (chiappa) only")
	}
	if status.FreeSpaceKB < 262144 {
		return status, fmt.Errorf("at least 256 MiB of free /home storage is required")
	}
	if status.XOVIInstalled && status.AppLoadInstalled && status.NodeInstalled &&
		status.PiInstalled && status.NativeDeps && status.QMLIndex &&
		status.XOVIPersistence && !forceRepair {
		return status, nil
	}

	stage, err := os.MkdirTemp("", "paper-agent-setup-*")
	if err != nil {
		return status, err
	}
	defer os.RemoveAll(stage)

	plan := planPrerequisites(status, forceRepair)
	var xoviPath, appLoadPath, rmShotPath, nodePath, tripleTapPath string
	if plan.xoviArchive {
		report(callback, "download", "Downloading and verifying XOVI…", 8)
		xoviPath, err = fetchDependency(ctx, client, dependencies.XOVI, stage)
		if err != nil {
			return status, err
		}
	}
	if plan.appLoad {
		report(callback, "download", "Downloading and verifying AppLoad…", 13)
		appLoadPath, err = fetchDependency(ctx, client, dependencies.AppLoad, stage)
		if err != nil {
			return status, err
		}
	}
	if plan.nativeBridge {
		report(callback, "download", "Downloading and verifying the native selection bridge…", 18)
		rmShotPath, err = fetchDependency(ctx, client, dependencies.RMShot, stage)
		if err != nil {
			return status, err
		}
	}
	if !status.NodeInstalled {
		report(callback, "download", "Downloading and verifying the ARM64 Node.js runtime…", 25)
		nodePath, err = fetchDependency(ctx, client, dependencies.Node, stage)
		if err != nil {
			return status, err
		}
	}
	if !status.XOVIPersistence {
		report(callback, "download", "Downloading and verifying XOVI persistence…", 30)
		tripleTapPath, err = fetchDependency(ctx, client, dependencies.TripleTap, stage)
		if err != nil {
			return status, err
		}
	}

	connection, err := reconnect(host, password, 3)
	if err != nil {
		return status, err
	}
	defer connection.Close()

	if plan.xoviArchive {
		report(callback, "device", "Uploading XOVI components…", 36)
		if err := connection.PushFile(xoviPath, "/tmp/paper-agent-xovi.tar.gz", 0o600); err != nil {
			return status, err
		}
		command := `set -eu
if [ ! -d /home/root/xovi ]; then
  tar -xzf /tmp/paper-agent-xovi.tar.gz -C /home/root
fi
EXT=/home/root/xovi/extensions.d
INACTIVE=/home/root/xovi/inactive-extensions
mkdir -p "$EXT"
for name in framebuffer-spy.so qt-command-executor.so; do
  if [ ! -f "$EXT/$name" ]; then
    if [ -f "$INACTIVE/$name" ]; then
      cp "$INACTIVE/$name" "$EXT/$name"
    else
      tar -xOzf /tmp/paper-agent-xovi.tar.gz "xovi/inactive-extensions/$name" >"$EXT/$name"
    fi
  fi
  chmod 0755 "$EXT/$name"
done
if [ ! -f "$EXT/xovi-message-broker.so" ]; then
  if [ -f "$INACTIVE/xovi-message-broker.so" ]; then
    mv "$INACTIVE/xovi-message-broker.so" "$EXT/xovi-message-broker.so"
  else
    tar -xOzf /tmp/paper-agent-xovi.tar.gz \
      xovi/inactive-extensions/xovi-message-broker.so >"$EXT/xovi-message-broker.so"
  fi
  chmod 0755 "$EXT/xovi-message-broker.so"
fi
rm -f /tmp/paper-agent-xovi.tar.gz
`
		if err := runChecked(connection, "install XOVI components", command); err != nil {
			return status, err
		}
	}

	if appLoadPath != "" {
		report(callback, "device", "Installing AppLoad…", 43)
		if err := connection.PushFile(appLoadPath, "/tmp/paper-agent-appload.zip", 0o600); err != nil {
			return status, err
		}
		command := `set -eu
cd /tmp
rm -rf paper-agent-appload
mkdir paper-agent-appload
(unzip -oq paper-agent-appload.zip -d paper-agent-appload || busybox unzip -o paper-agent-appload.zip -d paper-agent-appload)
test -f paper-agent-appload/appload.so
cp -f paper-agent-appload/appload.so /home/root/xovi/extensions.d/
mkdir -p /home/root/xovi/exthome/appload
if [ -d paper-agent-appload/shims ]; then
  cp -rf paper-agent-appload/shims /home/root/xovi/exthome/appload/
fi
if [ -d paper-agent-appload/exthome ]; then
  cp -rf paper-agent-appload/exthome/. /home/root/xovi/exthome/
fi
rm -rf paper-agent-appload paper-agent-appload.zip
`
		if err := runChecked(connection, "install AppLoad", command); err != nil {
			return status, err
		}
	}

	if rmShotPath != "" {
		report(callback, "device", "Installing the native selection dependencies…", 48)
		if err := connection.PushFile(rmShotPath, "/tmp/paper-agent-rm-shot.so", 0o755); err != nil {
			return status, err
		}
		if err := runChecked(connection, "install rm-shot", `set -eu
cp /tmp/paper-agent-rm-shot.so /home/root/xovi/extensions.d/rm-shot-aarch64.so
chmod 0755 /home/root/xovi/extensions.d/rm-shot-aarch64.so
rm -f /tmp/paper-agent-rm-shot.so
`); err != nil {
			return status, err
		}
	}

	if nodePath != "" {
		report(callback, "device", "Uploading the Node.js runtime…", 54)
		if err := connection.PushFile(nodePath, "/tmp/paper-agent-node.tar.gz", 0o600); err != nil {
			return status, err
		}
	}
	if !status.NodeInstalled || !status.PiInstalled {
		report(callback, "device", "Installing Node.js and Pi on the Move…", 62)
		command := fmt.Sprintf(`set -eu
umask 022
RUNTIME=/home/root/paper-agent/runtime
NODE_DIR="$RUNTIME/node-v%s"
if [ ! -x "$NODE_DIR/bin/node" ]; then
  test -f /tmp/paper-agent-node.tar.gz
  rm -rf "$NODE_DIR.staging"
  mkdir -p "$NODE_DIR.staging"
  tar -xzf /tmp/paper-agent-node.tar.gz -C "$NODE_DIR.staging" --strip-components=1
  mv "$NODE_DIR.staging" "$NODE_DIR"
fi
rm -f /tmp/paper-agent-node.tar.gz
if [ -e /home/root/node ] && [ ! -L /home/root/node ]; then
  echo "/home/root/node exists and is not a symlink" >&2
  exit 1
fi
ln -sfn "$NODE_DIR" /home/root/node
HOME=/home/root PATH="/home/root/node/bin:$PATH" \
  /home/root/node/bin/npm install --global --prefix /home/root/node \
  --omit=dev --no-audit --no-fund --cafile=/etc/ssl/cert.pem \
  "@earendil-works/pi-coding-agent@%s" "@earendil-works/pi-ai@%s"
test -x /home/root/node/bin/node
test -x /home/root/node/bin/pi
test -x /home/root/node/bin/pi-ai
`, nodeVersion, piVersion, piVersion)
		if err := runChecked(connection, "install Node.js and Pi", command); err != nil {
			return status, err
		}
	}

	if plan.qmlIndex {
		report(callback, "xovi", "Building the AppLoad QML index; this can take two minutes…", 70)
		if err := launchHashtableBuild(connection); err != nil {
			return status, err
		}
		connection.Close()
		connection = nil
		if err := waitForHashtable(ctx, host, password, callback); err != nil {
			return status, err
		}
		connection, err = reconnect(host, password, 20)
		if err != nil {
			return status, err
		}
		defer connection.Close()
	}

	if tripleTapPath != "" {
		report(callback, "xovi", "Installing triple-press XOVI persistence…", 80)
		if err := connection.PushFile(tripleTapPath, "/tmp/paper-agent-tripletap.tar.gz", 0o600); err != nil {
			return status, err
		}
		const installTripleTap = `set -eu
SOURCE=/tmp/paper-agent-tripletap-source
ARCHIVE=/tmp/paper-agent-tripletap.tar.gz
INSTALL=/home/root/xovi-tripletap
rm -rf "$SOURCE"
mkdir -p "$SOURCE" "$INSTALL"
tar -xzf "$ARCHIVE" -C "$SOURCE" --strip-components=1
test -x "$SOURCE/evtest.arm64"
test -f "$SOURCE/xovi-tripletap.service"
cp "$SOURCE/migrate-to-config.sh" "$SOURCE/config.default" "$INSTALL/"
chmod 0755 "$INSTALL/migrate-to-config.sh"
if [ ! -f "$INSTALL/config" ]; then
  "$INSTALL/migrate-to-config.sh"
fi
for name in main.sh enable.sh uninstall.sh version-switcher.sh init-version-switching.sh prepare-new-version.sh disable-version-switching.sh; do
  cp "$SOURCE/$name" "$INSTALL/$name"
  chmod 0755 "$INSTALL/$name"
done
cp "$SOURCE/evtest.arm64" "$INSTALL/evtest"
chmod 0755 "$INSTALL/evtest"
cp "$SOURCE/xovi-tripletap.service" "$INSTALL/xovi-tripletap.service"
printf '%s\n' 869497aa61435448bf0077fbf75fb264dcba92c5 >"$INSTALL/version.txt"
rm -rf "$SOURCE" "$ARCHIVE"
"$INSTALL/enable.sh"
`
		output, installErr := connection.Run(installTripleTap)
		if installErr != nil {
			return status, fmt.Errorf("install XOVI persistence: %w: %s", installErr, tail(output, 500))
		}
		if err := repairSSH(connection); err != nil {
			return status, err
		}
	}

	report(callback, "xovi", "Starting XOVI and AppLoad…", 88)
	output, startErr := connection.Run(`systemctl stop xovi-firststart 2>/dev/null || true
systemctl reset-failed xovi-firststart 2>/dev/null || true
systemd-run --unit=xovi-firststart --collect --service-type=oneshot /home/root/xovi/start
`)
	if startErr != nil && !strings.Contains(output, "already exists") {
		return status, fmt.Errorf("start XOVI: %w: %s", startErr, tail(output, 300))
	}
	connection.Close()
	connection = nil

	report(callback, "verify", "Verifying installed prerequisites…", 94)
	verified, err := inspectFresh(host, password, 30)
	if err != nil {
		return status, err
	}
	if !verified.XOVIInstalled || !verified.AppLoadInstalled || !verified.NodeInstalled ||
		!verified.PiInstalled || !verified.NativeDeps || !verified.QMLIndex ||
		!verified.XOVIPersistence {
		return verified, fmt.Errorf("prerequisite verification failed")
	}
	return verified, nil
}

func launchHashtableBuild(connection *device.Device) error {
	const script = `#!/bin/bash
set -u
xovi=/home/root/xovi
tab=$xovi/exthome/qt-resource-rebuilder/hashtab
systemctl stop xochitl 2>/dev/null || true
sleep 1
export XOVI_ROOT=/tmp/paper-agent-xovi-hashtab
rm -rf "$XOVI_ROOT"
mkdir -p "$XOVI_ROOT/extensions.d"
ln -s "$xovi/extensions.d/qt-resource-rebuilder.so" "$XOVI_ROOT/extensions.d/"
mkdir -p "$(dirname "$tab")"
rm -f "$tab"
QMLDIFF_HASHTAB_CREATE="$tab" QML_DISABLE_DISK_CACHE=1 \
  LD_PRELOAD="$xovi/xovi.so" /usr/bin/xochitl >/dev/null 2>&1 &
pid=$!
for i in $(seq 1 150); do [ -s "$tab" ] && break; sleep 1; done
sleep 3
kill "$pid" 2>/dev/null || true
sleep 2
kill -9 "$pid" 2>/dev/null || true
rm -rf "$XOVI_ROOT" /tmp/paper-agent-hashtab.sh
test -s "$tab"
`
	if err := connection.Push(strings.NewReader(script), "/tmp/paper-agent-hashtab.sh", 0o700); err != nil {
		return err
	}
	output, err := connection.Run(`systemctl stop paper-agent-hashtab 2>/dev/null || true
systemctl reset-failed paper-agent-hashtab 2>/dev/null || true
systemd-run --unit=paper-agent-hashtab --collect /bin/bash /tmp/paper-agent-hashtab.sh
`)
	if err != nil {
		return fmt.Errorf("launch QML index build: %w: %s", err, tail(output, 300))
	}
	return nil
}

func waitForHashtable(ctx context.Context, host, password string, callback Reporter) error {
	deadline := time.Now().Add(4 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(4 * time.Second):
		}
		connection, err := device.Connect(host, password)
		if err != nil {
			continue
		}
		output, _ := connection.Run(`printf 'unit='; systemctl is-active paper-agent-hashtab 2>/dev/null || true
test -s /home/root/xovi/exthome/qt-resource-rebuilder/hashtab && echo 'hashtab=1' || echo 'hashtab=0'
`)
		connection.Close()
		if strings.Contains(output, "hashtab=1") && !strings.Contains(output, "unit=active") {
			return nil
		}
		if strings.Contains(output, "unit=failed") {
			return fmt.Errorf("AppLoad QML index build failed")
		}
		report(callback, "xovi", "Still building the AppLoad QML index…", 74)
	}
	return fmt.Errorf("timed out while building the AppLoad QML index")
}

func repairSSH(connection *device.Device) error {
	const command = `set -eu
if ! mountpoint -q /etc; then
  mount -t overlay overlay -o lowerdir=/etc,upperdir=/var/volatile/etc,workdir=/var/volatile/.etc-work,uuid=on /etc \
    || mount -t overlay overlay -o lowerdir=/etc,upperdir=/var/volatile/etc,workdir=/var/volatile/.etc-work /etc
fi
if [ ! -e /etc/dropbear/dropbear_ed25519_host_key ]; then
  systemctl restart etc-dropbear.mount 2>/dev/null || true
fi
if [ ! -e /etc/dropbear/dropbear_ed25519_host_key ] && [ -e /home/root/.dropbear/dropbear_ed25519_host_key ]; then
  mount --bind /home/root/.dropbear /etc/dropbear
fi
mount -o remount,ro / 2>/dev/null || true
test -e /etc/dropbear/dropbear_ed25519_host_key
`
	return runChecked(connection, "repair the SSH key store", command)
}

func inspectFresh(host, password string, attempts int) (preflight.Status, error) {
	var last error
	for count := 0; count < attempts; count++ {
		connection, err := device.Connect(host, password)
		if err == nil {
			status, inspectErr := preflight.Inspect(connection, host)
			connection.Close()
			if inspectErr == nil {
				return status, nil
			}
			last = inspectErr
		} else {
			last = err
		}
		time.Sleep(2 * time.Second)
	}
	return preflight.Status{}, fmt.Errorf("verify prerequisites: %w", last)
}

// Deploy uploads one already verified release bundle and executes its
// transactional device installer.
func Deploy(connection *device.Device, bundlePath string) (string, error) {
	if filepath.Base(bundlePath) != "paper-agent-release.tar.gz" {
		return "", fmt.Errorf("unexpected release bundle filename")
	}
	if err := connection.PushFile(bundlePath, "/tmp/paper-agent-release.tar.gz", 0o600); err != nil {
		return "", err
	}
	output, err := connection.Run(`set -eu
rm -rf /tmp/paper-agent-release
mkdir -p /tmp/paper-agent-release
tar -xzf /tmp/paper-agent-release.tar.gz -C /tmp/paper-agent-release
rm -f /tmp/paper-agent-release.tar.gz
sh /tmp/paper-agent-release/install.sh
status=$?
rm -rf /tmp/paper-agent-release
exit "$status"
`)
	if err != nil {
		return output, fmt.Errorf("install Paper Agent release: %w: %s", err, tail(output, 700))
	}
	if !strings.Contains(output, "paper_agent=installed") {
		return output, fmt.Errorf("device did not confirm the Paper Agent installation")
	}
	return output, nil
}

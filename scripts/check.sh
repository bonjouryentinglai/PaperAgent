#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

if ! command -v node >/dev/null 2>&1; then
  echo "Node.js 18 or newer is required" >&2
  exit 127
fi
NODE_MAJOR=$(node -p 'Number(process.versions.node.split(".")[0])')
if [ "$NODE_MAJOR" -lt 18 ]; then
  echo "Node.js 18 or newer is required; found $(node --version)" >&2
  exit 1
fi
if ! command -v cargo >/dev/null 2>&1; then
  echo "Rust with Cargo and rustfmt is required" >&2
  exit 127
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go 1.26 or newer is required" >&2
  exit 127
fi

echo "Checking Rust formatting"
(
  cd device/native
  cargo fmt -- --check
)

echo "Running Rust tests"
(
  cd device/native
  cargo test --locked
)

echo "Checking and testing the AppLoad Settings backend"
(
  cd device/settings/backend
  cargo fmt -- --check
  cargo test --locked
)

echo "Checking and testing Node.js runtime"
node --check device/runtime/native-oracle-server.mjs
node device/runtime/native-oracle-server.mjs --self-test
node --check device/runtime/broker-signal.mjs
node device/runtime/broker-signal.mjs --self-test
node --check device/runtime/native-oracle-client.mjs
node --check device/runtime/scene.mjs
node --experimental-strip-types --check device/runtime/paper-agent-tools.ts
node --check device/runtime/rich-document.mjs
node --check device/runtime/image-generate.mjs
node --check device/runtime/layout-policy.mjs
node --check device/runtime/settings-controller.mjs
node --check installer/frontend/dist/app.js
node --test device/runtime/rich-document.test.mjs
node --test device/runtime/image-generate.test.mjs
node --test device/runtime/layout-policy.test.mjs
node --test device/runtime/settings-controller.test.mjs
node --test device/runtime/scene.test.mjs

echo "Checking shell syntax"
for script in scripts/*.sh; do
  bash -n "$script"
done
for script in device/runtime/*.sh; do
  sh -n "$script"
done
for script in device/systemd/*.sh; do
  sh -n "$script"
done
for script in device/settings/*.sh; do
  bash -n "$script"
done
for script in device/release/*.sh; do
  sh -n "$script"
done

node -e 'const fs=require("fs"); const m=JSON.parse(fs.readFileSync("device/settings/manifest.json")); if(m.id!=="paper-agent-settings"||m.loadsBackend!==true||m.entry!=="/ui/Main.qml") process.exit(1)'
grep -q '<file>ui/Main.qml</file>' device/settings/application.qrc
grep -q 'signal close' device/settings/ui/Main.qml
grep -q 'function unloading()' device/settings/ui/Main.qml

echo "Running desktop installer tests"
(
  cd installer
  go test ./...
)

echo "All off-device checks passed"

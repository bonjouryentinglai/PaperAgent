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

echo "Checking and testing Node.js runtime"
node --check device/runtime/native-oracle-server.mjs
node device/runtime/native-oracle-server.mjs --self-test
node --check device/runtime/broker-signal.mjs
node device/runtime/broker-signal.mjs --self-test
node --check device/runtime/native-oracle-client.mjs
node --check device/runtime/rich-document.mjs
node --check device/runtime/image-generate.mjs
node --check device/runtime/layout-policy.mjs
node --test device/runtime/rich-document.test.mjs
node --test device/runtime/image-generate.test.mjs
node --test device/runtime/layout-policy.test.mjs

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

echo "All off-device checks passed"

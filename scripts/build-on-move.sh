#!/usr/bin/env bash
set -euo pipefail

# Build and test the native crate on the Move, then download the binary. This
# script never installs or executes the resulting binary.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
CARGO_HOME_REMOTE=${PAPER_AGENT_CARGO_HOME:-/home/root/paper-agent/build-toolchain/cargo}
RUSTUP_HOME_REMOTE=${PAPER_AGENT_RUSTUP_HOME:-/home/root/paper-agent/build-toolchain/rustup}
TOOLCHAIN_REMOTE=${PAPER_AGENT_BUILD_TOOLCHAIN:-/home/root/paper-agent/build-toolchain}
STAGE=$(mktemp -d "${TMPDIR:-/tmp}/paper-agent-build.XXXXXX")

cleanup() {
  rm -rf "$STAGE"
}
trap cleanup EXIT

tar -czf "$STAGE/native-source.tar.gz" --exclude target -C "$ROOT" device/native
scp -O -o BatchMode=yes "$STAGE/native-source.tar.gz" "$DEVICE:/tmp/paper-agent-native-source.tar.gz"

ssh -o BatchMode=yes "$DEVICE" \
  "CARGO_HOME_REMOTE='$CARGO_HOME_REMOTE' RUSTUP_HOME_REMOTE='$RUSTUP_HOME_REMOTE' TOOLCHAIN_REMOTE='$TOOLCHAIN_REMOTE' sh -s" <<'DEVICE_SCRIPT'
set -eu
WORK=/tmp/paper-agent-native-build
rm -rf "$WORK"
mkdir -p "$WORK"
tar -xzf /tmp/paper-agent-native-source.tar.gz -C "$WORK"
rm -f /tmp/paper-agent-native-source.tar.gz

export CARGO_HOME="$CARGO_HOME_REMOTE"
export RUSTUP_HOME="$RUSTUP_HOME_REMOTE"
export CARGO_TARGET_DIR="$WORK/target"
export PATH="$CARGO_HOME/bin:$PATH"
export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER="$TOOLCHAIN_REMOTE/zig-cc"
export CC="$TOOLCHAIN_REMOTE/zig-cc" HOST_CC="$TOOLCHAIN_REMOTE/zig-cc"
export CC_aarch64_unknown_linux_gnu="$TOOLCHAIN_REMOTE/zig-cc"
export AR="$TOOLCHAIN_REMOTE/zig-ar" AR_aarch64_unknown_linux_gnu="$TOOLCHAIN_REMOTE/zig-ar"
export ZIG_GLOBAL_CACHE_DIR="$TOOLCHAIN_REMOTE/zig-cache/global"
export ZIG_LOCAL_CACHE_DIR="$WORK/zig-cache"
export CARGO_BUILD_JOBS=1

test -x "$CARGO_HOME/bin/cargo"
test -x "$TOOLCHAIN_REMOTE/zig-cc"
cd "$WORK/device/native"
cargo fmt -- --check
cargo test --locked --target aarch64-unknown-linux-gnu
cargo build --release --locked --target aarch64-unknown-linux-gnu
cp "$CARGO_TARGET_DIR/aarch64-unknown-linux-gnu/release/paper-agent-native" /tmp/paper-agent-native
chmod 0755 /tmp/paper-agent-native
DEVICE_SCRIPT

mkdir -p "$ROOT/dist"
scp -O -o BatchMode=yes "$DEVICE:/tmp/paper-agent-native" "$ROOT/dist/paper-agent-native"
ssh -o BatchMode=yes "$DEVICE" 'rm -f /tmp/paper-agent-native; rm -rf /tmp/paper-agent-native-build'
chmod 0755 "$ROOT/dist/paper-agent-native"
echo "Built $ROOT/dist/paper-agent-native"

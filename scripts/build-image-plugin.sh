#!/usr/bin/env bash
set -euo pipefail

# Cross-build the GPL-3.0 XOVI image bridge for Paper Pro / aarch64.

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SOURCE="$ROOT/device/xovi-image"
DIST="$ROOT/dist"
XOVI_SOURCE=${PAPER_AGENT_XOVI_SOURCE:-/tmp/paper-agent-xovi}
XOVI_COMMIT=${PAPER_AGENT_XOVI_COMMIT:-2b99649f5e4fd6288be7792a8570bd16418adb70}
TOOLCHAIN_IMAGE=${PAPER_AGENT_TOOLCHAIN_IMAGE:-eeems/remarkable-toolchain@sha256:297237d78a2aafa14896dd6a1495f7af173bcd646b0359be72ac00c04483dda3}

if [[ ! "$XOVI_COMMIT" =~ ^[0-9a-f]{40}$ ]]; then
  echo "PAPER_AGENT_XOVI_COMMIT must be a lowercase 40-character Git commit" >&2
  exit 1
fi

command -v docker >/dev/null
mkdir -p "$DIST"

DOCKER_ARGS=(--rm -v "$SOURCE:/build")
if [ -d "$XOVI_SOURCE/.git" ]; then
  DOCKER_ARGS+=(-v "$XOVI_SOURCE:/xovi-ro:ro")
fi

docker run "${DOCKER_ARGS[@]}" \
  -e PAPER_AGENT_XOVI_COMMIT="$XOVI_COMMIT" \
  "$TOOLCHAIN_IMAGE" bash -lc '
  set -euo pipefail
  cd /build
  . /opt/codex/*/*/environment-setup-*
  if [ -d /xovi-ro/.git ]; then
    actual_commit=$(git -C /xovi-ro rev-parse HEAD)
    if [ "$actual_commit" != "$PAPER_AGENT_XOVI_COMMIT" ]; then
      echo "mounted XOVI source is at $actual_commit; expected $PAPER_AGENT_XOVI_COMMIT" >&2
      exit 1
    fi
    if [ -n "$(git -C /xovi-ro status --porcelain)" ]; then
      echo "mounted XOVI source has local changes; refusing a non-reproducible build" >&2
      exit 1
    fi
    export XOVI_REPO=/xovi-ro
  else
    export XOVI_REPO=/tmp/xovi
    git init --quiet "$XOVI_REPO"
    git -C "$XOVI_REPO" remote add origin https://github.com/asivery/xovi
    git -C "$XOVI_REPO" fetch --quiet --depth=1 origin "$PAPER_AGENT_XOVI_COMMIT"
    git -C "$XOVI_REPO" checkout --quiet --detach FETCH_HEAD
    test "$(git -C "$XOVI_REPO" rev-parse HEAD)" = "$PAPER_AGENT_XOVI_COMMIT"
  fi
  python3 "$XOVI_REPO/util/xovigen.py" -o xovi.cpp -H xovi.h paper-agent-image.xovi
  qmake .
  make -j"$(nproc)"
  cp paper-agent-image.so /build/paper-agent-image-aarch64.so
  make clean || true
  rm -f paper-agent-image.so Makefile .qmake.stash xovi.cpp xovi.h moc_predefs.h
'

mv "$SOURCE/paper-agent-image-aarch64.so" "$DIST/paper-agent-image-aarch64.so"
echo "Built $DIST/paper-agent-image-aarch64.so"

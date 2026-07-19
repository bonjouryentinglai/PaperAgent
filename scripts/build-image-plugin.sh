#!/usr/bin/env bash
set -euo pipefail

# Cross-build the GPL-3.0 XOVI image bridge for Paper Pro / aarch64.

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SOURCE="$ROOT/device/xovi-image"
DIST="$ROOT/dist"
XOVI_SOURCE=${PAPER_AGENT_XOVI_SOURCE:-/tmp/paper-agent-xovi}

command -v docker >/dev/null
mkdir -p "$DIST"

DOCKER_ARGS=(--rm -v "$SOURCE:/build")
if [ -d "$XOVI_SOURCE/.git" ]; then
  DOCKER_ARGS+=(-v "$XOVI_SOURCE:/xovi-ro:ro")
fi

docker run "${DOCKER_ARGS[@]}" eeems/remarkable-toolchain:latest-rmpp bash -lc '
  set -euo pipefail
  cd /build
  . /opt/codex/*/*/environment-setup-*
  if [ -d /xovi-ro/.git ]; then
    export XOVI_REPO=/xovi-ro
  else
    export XOVI_REPO=/tmp/xovi
    git clone --depth=1 https://github.com/asivery/xovi "$XOVI_REPO"
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

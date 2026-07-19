#!/bin/sh
set -eu

BASE=/home/root/paper-agent/native
ENV_FILE=/home/root/paper-agent/config.env

if [ -f "$ENV_FILE" ]; then
  set -a
  . "$ENV_FILE"
  set +a
fi

PI_BIN_DIR=${PAPER_AGENT_PI_BIN_DIR:-/home/root/node/bin}
export PAPER_AGENT_PI_BIN_DIR="$PI_BIN_DIR"
export PAPER_AGENT_PROVIDER=${PAPER_AGENT_PROVIDER:-openai-codex}
export PAPER_AGENT_MODEL=${PAPER_AGENT_MODEL:-gpt-5.6-sol}
export PAPER_AGENT_THINKING=${PAPER_AGENT_THINKING:-off}
export PAPER_AGENT_CJK_SCALE=${PAPER_AGENT_CJK_SCALE:-0.78}
export HOME=/home/root
export PATH="$PI_BIN_DIR:$PATH"

test -x "$PI_BIN_DIR/node"
test -x "$PI_BIN_DIR/pi"
test -f "$BASE/native-oracle-server.mjs"
test -x "$BASE/paper-agent-native"
mkdir -p "$BASE/oracle-data" "$BASE/jobs" "$BASE/artifacts"
chmod 0700 "$BASE/oracle-data" "$BASE/jobs" "$BASE/artifacts"

exec "$PI_BIN_DIR/node" "$BASE/native-oracle-server.mjs"

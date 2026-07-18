#!/usr/bin/env bash
set -euo pipefail

# Run Pi's interactive OpenAI/Codex device login on the Move. Tokens are saved
# only in Pi's private credential store on the tablet.

HOST=${PAPER_AGENT_HOST:?Set PAPER_AGENT_HOST to the Move hostname or IP}
DEVICE_USER=${PAPER_AGENT_DEVICE_USER:-root}
DEVICE="${DEVICE_USER}@${HOST}"

exec ssh -t "$DEVICE" '
  set -eu
  umask 077
  AUTH_DIR=/home/root/.pi/agent
  mkdir -p "$AUTH_DIR"
  chmod 700 /home/root/.pi "$AUTH_DIR" 2>/dev/null || true
  cd "$AUTH_DIR"
  HOME=/home/root PATH="/home/root/node/bin:$PATH" \
    /home/root/node/bin/pi-ai login openai-codex
  chmod 600 "$AUTH_DIR/auth.json"
  echo "ChatGPT subscription credential saved in Pi on the Move."
'

#!/bin/sh
set -eu

# Complete one explicit Xochitl selection action:
# capture PNG -> selected Paper Agent action -> bounded stroke job -> guarded native ink.
# This wrapper is intentionally separate from native-selection-prepare.sh so
# the renderer/coordinator remains testable without opening the Marker device.

HAS_SCENE_TARGET=0
if [ "$#" -eq 5 ]; then
  # Compatibility during a runtime-first upgrade from the one-button QMD.
  ACTION=ai
  NEW_PAGE_REQUIRED=0
elif [ "$#" -eq 6 ]; then
  ACTION=$1
  shift
  NEW_PAGE_REQUIRED=0
elif [ "$#" -eq 7 ]; then
  ACTION=$1
  NEW_PAGE_REQUIRED=$7
  set -- "$2" "$3" "$4" "$5" "$6"
elif [ "$#" -eq 15 ]; then
  ACTION=$1
  NEW_PAGE_REQUIRED=$7
  SCENE_X=$8
  SCENE_Y=$9
  SCENE_WIDTH=${10}
  SCENE_HEIGHT=${11}
  PAPER_X=${12}
  PAPER_Y=${13}
  PAPER_WIDTH=${14}
  PAPER_HEIGHT=${15}
  HAS_SCENE_TARGET=1
  set -- "$2" "$3" "$4" "$5" "$6"
else
  echo "usage: $0 ACTION SELECTION.png X Y WIDTH HEIGHT NEW_PAGE_REQUIRED [SCENE_X SCENE_Y SCENE_WIDTH SCENE_HEIGHT PAPER_X PAPER_Y PAPER_WIDTH PAPER_HEIGHT]" >&2
  exit 2
fi

PNG=$1
X=$2
Y=$3
WIDTH=$4
HEIGHT=$5
BASE=/home/root/paper-agent/native
JOBS="$BASE/jobs"
PREPARE="$BASE/native-selection-prepare.sh"
BIN="$BASE/paper-agent-native"
NODE=/home/root/node/bin/node
STREAM_CLIENT="$BASE/native-oracle-client.mjs"
BROKER_SIGNAL="$BASE/broker-signal.mjs"
LOCK=/run/paper-agent-native-coordinator.lock
FINAL_STATUS=error

case "$ACTION" in
  ai|beautify) ;;
  *) echo "unsupported Paper Agent action: $ACTION" >&2; exit 2 ;;
esac
case "$NEW_PAGE_REQUIRED" in
  0|1) ;;
  *) echo "new-page policy must be 0 or 1" >&2; exit 2 ;;
esac

send_broker() {
  if [ -x "$NODE" ] && [ -f "$BROKER_SIGNAL" ]; then
    "$NODE" "$BROKER_SIGNAL" "$1" "$2" >/dev/null 2>&1 || true
  fi
}

send_status() {
  send_broker 'paper-agent$status' "$1"
}

send_request_state() {
  send_broker 'paper-agent$request' "$REQUEST_ID,$1"
}

current_xochitl_pid() {
  systemctl show xochitl -p MainPID --value 2>/dev/null || true
}

xochitl_session_alive() {
  current=$(current_xochitl_pid)
  [ "$current" = "$XOCHITL_PID" ] && [ -r "/proc/$XOCHITL_PID/comm" ] \
    && [ "$(cat "/proc/$XOCHITL_PID/comm" 2>/dev/null || true)" = xochitl ]
}

ensure_primary_pen() {
  sequence=$1
  ack="/run/paper-agent-tool-$REQUEST_ID-$sequence.ack"
  rm -f "$ack"
  attempt=0
  while [ "$attempt" -lt 34 ]; do
    if ! xochitl_session_alive; then
      echo "Xochitl restarted during the native request" >&2
      return 1
    fi
    send_broker 'paper-agent$tool' "primary,$REQUEST_ID,$sequence"
    sleep 0.08
    if [ -f "$ack" ] && [ ! -L "$ack" ]; then
      rm -f "$ack"
      sleep 0.1
      return 0
    fi
    attempt=$((attempt + 1))
  done
  echo "Xochitl did not confirm the primary pen" >&2
  return 1
}

case "$PNG" in
  /home/root/paper-agent/selection/native-selection-*.png) ;;
  *) echo "refusing unexpected selection path: $PNG" >&2; exit 2 ;;
esac
REQUEST_ID=${PNG##*/native-selection-}
REQUEST_ID=${REQUEST_ID%.png}
case "$REQUEST_ID" in
  *[!0-9]*|'') echo "selection request id is invalid" >&2; exit 2 ;;
esac
case "$X:$Y:$WIDTH:$HEIGHT" in
  *[!0-9:]*|'') echo "placement must contain non-negative integers" >&2; exit 2 ;;
esac

umask 077
mkdir -p "$JOBS"
XOCHITL_PID=$(current_xochitl_pid)
case "$XOCHITL_PID" in
  ''|0|*[!0-9]*) echo "Xochitl has no stable main process" >&2; exit 1 ;;
esac
if ! mkdir "$LOCK" 2>/dev/null; then
  echo "another native selection request is already active" >&2
  send_status error
  send_request_state error
  exit 1
fi
JOB=
cleanup() {
  rm -f "$PNG"
  if [ -n "$JOB" ]; then
    rm -f "$JOB"
  fi
  rmdir "$LOCK" 2>/dev/null || true
  send_status "$FINAL_STATUS"
  # XOVI's FIFO broker consumes only the first line from each read. Keep the
  # request lifecycle signal out of the same read as the final status.
  sleep 0.12
  send_request_state "$FINAL_STATUS"
}
trap cleanup EXIT INT TERM
send_status thinking
sleep 0.08
send_status thinking
# BusyBox mktemp requires the six trailing X characters to be the final
# template characters; a suffix such as `.XXXXXX.strokes` is rejected on Move.
JOB=$(mktemp "$JOBS/native-selection.XXXXXX")

test -x "$PREPARE" || { echo "native coordinator is missing: $PREPARE" >&2; exit 1; }
test -x "$BIN" || { echo "native writer is missing: $BIN" >&2; exit 1; }

# rm-shot is delayed and may outlive the transient QML selection scene. Wait
# until the file has a stable non-zero size before handing it to Pi.
previous_size=-1
stable_count=0
attempt=0
while [ "$attempt" -lt 40 ]; do
  if [ -s "$PNG" ]; then
    size=$(stat -c %s "$PNG" 2>/dev/null || echo 0)
    if [ "$size" -gt 0 ] && [ "$size" -eq "$previous_size" ]; then
      stable_count=$((stable_count + 1))
      if [ "$stable_count" -ge 2 ]; then
        break
      fi
    else
      stable_count=0
    fi
    previous_size=$size
  fi
  attempt=$((attempt + 1))
  sleep 0.2
done
test -s "$PNG" || { echo "selection screenshot was not completed" >&2; exit 1; }
test "$stable_count" -ge 2 || { echo "selection screenshot did not settle" >&2; exit 1; }

# Chiappa's BusyBox od omits GNU's -A/-N options. BusyBox hexdump supports a
# length bound and an explicit byte format, producing the same stable value.
magic=$(hexdump -n 8 -v -e '1/1 "%02x"' "$PNG")
test "$magic" = "89504e470d0a1a0a" || { echo "selection screenshot is not PNG" >&2; exit 1; }
if ! xochitl_session_alive; then
  echo "Xochitl restarted while the selection screenshot was captured" >&2
  exit 1
fi

# Prefer the resident Pi RPC service. Exit 75 means the service was unavailable
# before accepting this request, so the proven one-shot path remains a safe
# fallback. Any error after acceptance may follow partial native ink and must
# never be replayed automatically.
# Never let the resident oracle start while Xochitl still owns the lasso tool.
# The oracle repeats this acknowledged guard immediately before every job.
ensure_primary_pen 0
if [ -x "$NODE" ] && [ -f "$STREAM_CLIENT" ]; then
  if [ "$HAS_SCENE_TARGET" -eq 1 ]; then
    if PAPER_AGENT_XOCHITL_PID="$XOCHITL_PID" \
        "$NODE" "$STREAM_CLIENT" "$ACTION" "$PNG" "$X" "$Y" "$WIDTH" "$HEIGHT" \
        "$NEW_PAGE_REQUIRED" "$SCENE_X" "$SCENE_Y" "$SCENE_WIDTH" "$SCENE_HEIGHT" \
        "$PAPER_X" "$PAPER_Y" "$PAPER_WIDTH" "$PAPER_HEIGHT"; then
      stream_rc=0
    else
      stream_rc=$?
    fi
  else
    if PAPER_AGENT_XOCHITL_PID="$XOCHITL_PID" \
        "$NODE" "$STREAM_CLIENT" "$ACTION" "$PNG" "$X" "$Y" "$WIDTH" "$HEIGHT" "$NEW_PAGE_REQUIRED"; then
      stream_rc=0
    else
      stream_rc=$?
    fi
  fi
  if [ "$stream_rc" -eq 0 ]; then
    FINAL_STATUS=done
    echo "native_writeback=streaming-complete"
    exit 0
  elif [ "$stream_rc" -eq 130 ]; then
    FINAL_STATUS=cancelled
    echo "native_writeback=cancelled"
    exit 0
  else
    if [ "$stream_rc" -ne 75 ]; then
      echo "resident native oracle failed after accepting the request" >&2
      exit "$stream_rc"
    fi
    echo "resident native oracle unavailable; using one-shot fallback" >&2
  fi
fi

if [ "$NEW_PAGE_REQUIRED" -eq 1 ]; then
  echo "resident native oracle is required for safe new-page placement" >&2
  exit 1
fi

"$PREPARE" "$ACTION" "$PNG" "$X" "$Y" "$WIDTH" "$HEIGHT" "$JOB"
# The one-shot fallback has no resident oracle, so repeat the same acknowledged
# guard immediately before its direct Marker write.
ensure_primary_pen 4096
PAPER_AGENT_XOCHITL_PID="$XOCHITL_PID" \
  "$BIN" write "$JOB" --confirm PAPER_AGENT_NATIVE_WRITE_V1
FINAL_STATUS=done
echo "native_writeback=complete"

#!/bin/sh
set -eu

# Complete one explicit Xochitl selection action:
# capture PNG -> selected Paper Agent action -> bounded stroke job -> guarded native ink.
# This wrapper is intentionally separate from native-selection-prepare.sh so
# the renderer/coordinator remains testable without opening the Marker device.

if [ "$#" -eq 5 ]; then
  # Compatibility during a runtime-first upgrade from the one-button QMD.
  ACTION=ai
elif [ "$#" -eq 6 ]; then
  ACTION=$1
  shift
else
  echo "usage: $0 ACTION SELECTION.png X Y WIDTH HEIGHT" >&2
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
LOCK=/run/paper-agent-native-coordinator.lock
FINAL_STATUS=error

case "$ACTION" in
  ai|beautify) ;;
  *) echo "unsupported Paper Agent action: $ACTION" >&2; exit 2 ;;
esac

send_status() {
  if [ -p /run/xovi-mb ]; then
    printf 'upaper-agent$status:%s\n' "$1" > /run/xovi-mb || true
  fi
}

send_request_state() {
  if [ -p /run/xovi-mb ]; then
    printf 'upaper-agent$request:%s,%s\n' "$REQUEST_ID" "$1" > /run/xovi-mb || true
  fi
}

restore_primary_pen() {
  if [ -p /run/xovi-mb ]; then
    printf 'upaper-agent$tool:primary\n' > /run/xovi-mb || true
  fi
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
  send_request_state "$FINAL_STATUS"
}
trap cleanup EXIT INT TERM
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

# Prefer the resident Pi RPC service. Exit 75 means the service was unavailable
# before accepting this request, so the proven one-shot path remains a safe
# fallback. Any error after acceptance may follow partial native ink and must
# never be replayed automatically.
# AI and Beautify both dismiss the lasso and write below it. Repeat the tool
# restore from the detached worker so both actions use the same proven path.
restore_primary_pen
if [ -x "$NODE" ] && [ -f "$STREAM_CLIENT" ]; then
  if "$NODE" "$STREAM_CLIENT" "$ACTION" "$PNG" "$X" "$Y" "$WIDTH" "$HEIGHT"; then
    FINAL_STATUS=done
    echo "native_writeback=streaming-complete"
    exit 0
  else
    stream_rc=$?
    if [ "$stream_rc" -ne 75 ]; then
      echo "resident native oracle failed after accepting the request" >&2
      exit "$stream_rc"
    fi
    echo "resident native oracle unavailable; using one-shot fallback" >&2
  fi
fi

"$PREPARE" "$ACTION" "$PNG" "$X" "$Y" "$WIDTH" "$HEIGHT" "$JOB"
# The QMD normally restores the pen as soon as the selection closes. Repeat
# the request immediately before writing so a slow UI transition cannot leave
# the injected Marker events assigned to the lasso tool.
restore_primary_pen
sleep 0.15
"$BIN" write "$JOB" --confirm PAPER_AGENT_NATIVE_WRITE_V1
FINAL_STATUS=done
echo "native_writeback=complete"

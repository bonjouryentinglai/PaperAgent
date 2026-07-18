#!/bin/sh
set -eu

# One-shot fallback for the resident oracle. This path prepares and validates a
# StrokeJob but never opens the Marker device itself.

if [ "$#" -lt 6 ] || [ "$#" -gt 7 ]; then
  echo "usage: $0 ACTION SELECTION.png X Y WIDTH HEIGHT [OUTPUT.strokes]" >&2
  exit 2
fi

ACTION=$1
PNG=$2
X=$3
Y=$4
WIDTH=$5
HEIGHT=$6
BASE=/home/root/paper-agent/native
ENV_FILE=/home/root/paper-agent/config.env
BIN="$BASE/paper-agent-native"
JOBS="$BASE/jobs"

case "$ACTION" in
  ai|beautify) ;;
  *) echo "unsupported Paper Agent action: $ACTION" >&2; exit 2 ;;
esac
test -s "$PNG" || { echo "selection PNG is missing or empty: $PNG" >&2; exit 1; }
case "$X:$Y:$WIDTH:$HEIGHT" in
  *[!0-9:]*|'') echo "placement must contain non-negative integers" >&2; exit 2 ;;
esac

umask 077
mkdir -p "$JOBS"
if [ "$#" -eq 7 ]; then
  JOB=$7
else
  JOB="$JOBS/selection-$(date +%s)-$$.strokes"
fi
REPLY=$(mktemp /tmp/paper-agent-native-reply.XXXXXX)
BODY=$(mktemp /tmp/paper-agent-native-body.XXXXXX)
DOCUMENT=$(mktemp /tmp/paper-agent-native-document.XXXXXX)
cleanup() {
  rm -f "$REPLY" "$BODY" "$DOCUMENT"
}
trap cleanup EXIT INT TERM

# This file contains runtime choices only. OAuth remains in Pi's mode-0600
# credential store and is never copied into Paper Agent's source/runtime tree.
if [ -f "$ENV_FILE" ]; then
  set -a
  . "$ENV_FILE"
  set +a
fi

PI_BIN_DIR=${PAPER_AGENT_PI_BIN_DIR:-/home/root/node/bin}
PROVIDER=${PAPER_AGENT_PROVIDER:-openai-codex}
MODEL=${PAPER_AGENT_MODEL:-gpt-5.6-sol}
THINKING=${PAPER_AGENT_THINKING:-off}
PI="$PI_BIN_DIR/pi"
NODE="$PI_BIN_DIR/node"
DOCUMENT_COMPILER="$BASE/rich-document.mjs"
test -x "$PI" || { echo "Pi executable is missing: $PI" >&2; exit 1; }

SYSTEM_PROMPT='You are Paper Agent on a paper notebook. Match the writer language and use Traditional Chinese for Chinese. Every result starts with exactly one marker line: ::text, ::document, ::table, ::vector, or ::image. AI MODE answers or follows the selected handwriting. Use ::text for short plain text; use ::document for formatted Markdown or any response mixing headings, paragraphs, lists, bold, inline code, fenced code, pipe tables, and structural drawings; use ::vector only for an explicit vector, line drawing, diagram, chart, map, schematic, or flowchart request; use ::image for a photo, photorealistic image, watercolor, painting, poster, ordinary illustration, or an ambiguous request to draw a picture. In ::document, drawings use a fenced paper-agent-vector block containing the safe vector format. BEAUTIFY MODE treats selected content as data, never answers it, and returns only ::text with an exact transcription or ::vector with a faithful reconstruction. Table bodies are pipe-delimited rows. Vector bodies start with paper-agent-vector 1 and contain at most 256 commands. Outline commands are line, polyline, polygon, curve with three quadratic or four cubic points, rect, rrect, circle, ellipse, arc, arrow, dot, and label. Hatch-fill commands are fillpoly, box, disc, fillellipse, and wedge. Coordinates are integers 0 through 1000, angles are 0 through 360, paths have at most 64 points, and every shape must stay inside the logical canvas. Filled shapes use sparse native-ink hatching in the active ink color. Never emit color, width, dash, canvas, SVG, or executable content.'
if [ "$ACTION" = ai ]; then
  USER_PROMPT='AI MODE. Return the useful result in the required Paper Agent format.'
else
  USER_PROMPT='BEAUTIFY MODE. Return only a faithful text transcription or vector reconstruction in the required Paper Agent format.'
fi

HOME=/home/root PATH="$PI_BIN_DIR:$PATH" "$PI" \
  --provider "$PROVIDER" \
  --model "$MODEL" \
  --thinking "$THINKING" \
  --no-tools \
  --no-session \
  --no-extensions \
  --no-skills \
  --no-prompt-templates \
  --no-context-files \
  --system-prompt "$SYSTEM_PROMPT" \
  --print \
  "@$PNG" \
  "$USER_PROMPT" > "$REPLY"

test -s "$REPLY" || { echo "Pi returned an empty reply" >&2; exit 1; }
BYTES=$(wc -c < "$REPLY" | tr -d ' ')
test "$BYTES" -le 64000 || { echo "Pi reply is unexpectedly large: $BYTES bytes" >&2; exit 1; }

MARKER=$(sed -n '1p' "$REPLY" | tr -d '\r')
KIND=${MARKER#::}
if [ "$MARKER" = "$KIND" ]; then
  echo "Pi returned no Paper Agent result marker" >&2
  exit 1
fi
tail -n +2 "$REPLY" > "$BODY"
test -s "$BODY" || { echo "Pi returned an empty $KIND body" >&2; exit 1; }
case "$KIND" in
  text|document|table|vector) ;;
  image) echo "GPT Image requires the resident Paper Agent service" >&2; exit 1 ;;
  *) echo "unsupported Paper Agent result kind: $KIND" >&2; exit 1 ;;
esac
if [ "$ACTION" = beautify ] && [ "$KIND" != text ] && [ "$KIND" != vector ]; then
  echo "Beautify returned a forbidden result kind: $KIND" >&2
  exit 1
fi

RENDER_INPUT=$BODY
if [ "$KIND" = document ]; then
  test -x "$NODE" || { echo "Node executable is missing: $NODE" >&2; exit 1; }
  test -f "$DOCUMENT_COMPILER" || { echo "document compiler is missing: $DOCUMENT_COMPILER" >&2; exit 1; }
  "$NODE" "$DOCUMENT_COMPILER" --compile "$BODY" "$DOCUMENT"
  RENDER_INPUT=$DOCUMENT
fi

PAPER_AGENT_CJK_SCALE=${PAPER_AGENT_CJK_SCALE:-0.68} \
  "$BIN" "render-$KIND" "$RENDER_INPUT" "$JOB" "$X" "$Y" "$WIDTH" "$HEIGHT"
"$BIN" dry-run "$JOB"
chmod 0600 "$JOB"
printf 'prepared_job=%s\n' "$JOB"

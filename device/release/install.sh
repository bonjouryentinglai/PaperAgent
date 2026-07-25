#!/bin/sh
set -eu

# Device-side transactional installer for a checksum-verified Paper Agent
# release bundle. The desktop installer uploads and extracts the bundle below
# /tmp, then executes this file. Updates and repairs use the same transaction.

RELEASE_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PAYLOAD="$RELEASE_ROOT/payload"
BASE=/home/root/paper-agent
NATIVE="$BASE/native"
ICONS="$BASE/assets/icons"
QMD_HOME=/home/root/xovi/exthome/qt-resource-rebuilder
QMD_FILE="$QMD_HOME/paperAgentSelection.qmd"
SYSTEMD_HOME="$BASE/systemd"
UNIT_SOURCE="$SYSTEMD_HOME/paper-agent-native-oracle.service"
UNIT=/run/systemd/system/paper-agent-native-oracle.service
START_HOOK=/home/root/xovi/scripts/post-start/paper-agent-native-oracle.sh
IMAGE_PLUGIN=/home/root/xovi/extensions.d/paper-agent-image.so
SETTINGS_APP=/home/root/xovi/exthome/appload/paper-agent-settings
CONFIG="$BASE/config.env"
VERSION_FILE="$BASE/VERSION"
STATE_ROOT="$BASE/backups"
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="$STATE_ROOT/install-$STAMP"
INCOMING="$BASE/.install-incoming.$$"
FILES='paper-agent-native broker-signal.mjs native-oracle-client.mjs native-oracle-server.mjs native-oracle-service.sh native-selection-prepare.sh native-selection-write.sh rich-document.mjs rich-document.test.mjs image-generate.mjs image-generate.test.mjs layout-policy.mjs layout-policy.test.mjs settings-controller.mjs settings-controller.test.mjs scene.mjs scene.test.mjs paper-agent-tools.ts'
SKILLS='ai-selection structured-drawing beautify-selection'

test -f "$RELEASE_ROOT/VERSION"
RELEASE_VERSION=$(tr -d '\r\n' <"$RELEASE_ROOT/VERSION")
case "$RELEASE_VERSION" in
  ''|*[!0-9A-Za-z._+-]*) echo "Invalid release version" >&2; exit 1 ;;
esac
test -x "$PAYLOAD/paper-agent-native"
test -x "$PAYLOAD/paper-agent-image.so"
test -f "$PAYLOAD/paper-agent-native-oracle.service"
test -f "$PAYLOAD/paper-agent-xovi-post-start.sh"
test -f "$PAYLOAD/paper-agent-selection.qmd"
test -f "$PAYLOAD/paper-agent.env.example"
test -f "$PAYLOAD/settings-app/manifest.json"
test -f "$PAYLOAD/settings-app/resources.rcc"
test -f "$PAYLOAD/settings-app/icon.png"
test -x "$PAYLOAD/settings-app/backend/entry"

test -x /home/root/node/bin/node
test -x /home/root/node/bin/pi
test -f /home/root/.pi/agent/auth.json
test -d "$QMD_HOME"
test -f /home/root/xovi/extensions.d/appload.so
for dep in framebuffer-spy.so rm-shot-aarch64.so qt-command-executor.so xovi-message-broker.so; do
  test -f "/home/root/xovi/extensions.d/$dep"
done

cleanup() {
  rm -rf "$INCOMING"
  rm -f /tmp/paper-agent-health.log
}
TRANSACTION_READY=0
COMMITTED=0
on_exit() {
  status=$?
  trap - EXIT
  if [ "$status" -ne 0 ] && [ "$TRANSACTION_READY" -eq 1 ] && [ "$COMMITTED" -eq 0 ]; then
    rollback
  fi
  cleanup
  exit "$status"
}
trap on_exit EXIT
mkdir -p "$INCOMING" "$STATE_ROOT" "$BACKUP" "$NATIVE" "$NATIVE/skills" \
  "$ICONS" "$BASE/selection" "$SYSTEMD_HOME" "$(dirname "$SETTINGS_APP")"
cp -a "$PAYLOAD"/. "$INCOMING"/

backup_file() {
  name=$1
  path=$2
  if [ -f "$path" ] || [ -L "$path" ]; then
    cp -p "$path" "$BACKUP/$name"
    echo present >"$BACKUP/$name.state"
  else
    echo absent >"$BACKUP/$name.state"
  fi
}

restore_file() {
  name=$1
  path=$2
  if [ "$(cat "$BACKUP/$name.state")" = present ]; then
    mkdir -p "$(dirname "$path")"
    cp -p "$BACKUP/$name" "$path"
  else
    rm -f "$path"
  fi
}

backup_dir() {
  name=$1
  path=$2
  if [ -d "$path" ]; then
    cp -a "$path" "$BACKUP/$name"
    echo present >"$BACKUP/$name.state"
  else
    echo absent >"$BACKUP/$name.state"
  fi
}

restore_dir() {
  name=$1
  path=$2
  rm -rf "$path"
  if [ "$(cat "$BACKUP/$name.state")" = present ]; then
    mkdir -p "$(dirname "$path")"
    cp -a "$BACKUP/$name" "$path"
  fi
}

for name in $FILES; do backup_file "$name" "$NATIVE/$name"; done
for skill in $SKILLS; do backup_file "skill-$skill" "$NATIVE/skills/$skill/SKILL.md"; done
backup_file paper-agent-ai.svg "$ICONS/paper-agent-ai.svg"
backup_file paper-agent-beautify.svg "$ICONS/paper-agent-beautify.svg"
backup_file paper-agent-native-oracle.service "$UNIT"
backup_file paper-agent-native-oracle.source "$UNIT_SOURCE"
backup_file paper-agent-native-oracle.hook "$START_HOOK"
backup_file paper-agent-image.so "$IMAGE_PLUGIN"
backup_file paperAgentSelection.qmd "$QMD_FILE"
backup_file config.env "$CONFIG"
backup_file VERSION "$VERSION_FILE"
backup_dir paper-agent-settings "$SETTINGS_APP"
OLD_ENABLED=$(systemctl is-enabled paper-agent-native-oracle.service 2>/dev/null || true)
OLD_ACTIVE=$(systemctl is-active paper-agent-native-oracle.service 2>/dev/null || true)
printf '%s\n' "$OLD_ENABLED" >"$BACKUP/service.enabled"
printf '%s\n' "$OLD_ACTIVE" >"$BACKUP/service.active"
TRANSACTION_READY=1

rollback() {
  set +e
  systemctl stop paper-agent-native-oracle.service 2>/dev/null || true
  for name in $FILES; do restore_file "$name" "$NATIVE/$name"; done
  for skill in $SKILLS; do
    mkdir -p "$NATIVE/skills/$skill"
    restore_file "skill-$skill" "$NATIVE/skills/$skill/SKILL.md"
  done
  restore_file paper-agent-ai.svg "$ICONS/paper-agent-ai.svg"
  restore_file paper-agent-beautify.svg "$ICONS/paper-agent-beautify.svg"
  restore_file paper-agent-native-oracle.service "$UNIT"
  restore_file paper-agent-native-oracle.source "$UNIT_SOURCE"
  restore_file paper-agent-native-oracle.hook "$START_HOOK"
  restore_file paper-agent-image.so "$IMAGE_PLUGIN"
  restore_file paperAgentSelection.qmd "$QMD_FILE"
  restore_file config.env "$CONFIG"
  restore_file VERSION "$VERSION_FILE"
  restore_dir paper-agent-settings "$SETTINGS_APP"
  systemctl daemon-reload || true
  if [ "$(cat "$BACKUP/service.enabled")" = enabled ]; then
    systemctl enable paper-agent-native-oracle.service >/dev/null 2>&1 || true
  else
    systemctl disable paper-agent-native-oracle.service >/dev/null 2>&1 || true
  fi
  if [ "$(cat "$BACKUP/service.active")" = active ]; then
    systemctl start paper-agent-native-oracle.service || true
  fi
  /home/root/xovi/start || true
}

cp "$INCOMING/paper-agent-native" "$NATIVE/paper-agent-native"
cp "$INCOMING/paper-agent-image.so" "$IMAGE_PLUGIN"
cp "$INCOMING/assets/icons/paper-agent-ai.svg" "$ICONS/paper-agent-ai.svg"
cp "$INCOMING/assets/icons/paper-agent-beautify.svg" "$ICONS/paper-agent-beautify.svg"
grep -q '<svg' "$ICONS/paper-agent-ai.svg"
grep -q '<svg' "$ICONS/paper-agent-beautify.svg"
for name in broker-signal.mjs native-oracle-client.mjs native-oracle-server.mjs native-oracle-service.sh native-selection-prepare.sh native-selection-write.sh rich-document.mjs rich-document.test.mjs image-generate.mjs image-generate.test.mjs layout-policy.mjs layout-policy.test.mjs settings-controller.mjs settings-controller.test.mjs scene.mjs scene.test.mjs paper-agent-tools.ts; do
  cp "$INCOMING/runtime/$name" "$NATIVE/$name"
done
for skill in $SKILLS; do
  mkdir -p "$NATIVE/skills/$skill"
  cp "$INCOMING/runtime/skills/$skill/SKILL.md" "$NATIVE/skills/$skill/SKILL.md"
done
cp "$INCOMING/paper-agent-native-oracle.service" "$UNIT_SOURCE"
cp "$INCOMING/paper-agent-xovi-post-start.sh" "$START_HOOK"
ln -sf "$UNIT_SOURCE" "$UNIT"
cp "$INCOMING/paper-agent-selection.qmd" "$QMD_FILE"
if [ ! -f "$CONFIG" ]; then cp "$INCOMING/paper-agent.env.example" "$CONFIG"; fi
if grep -Eqx 'PAPER_AGENT_CJK_SCALE=(0\.78|0\.86)' "$CONFIG"; then
  sed -Ei 's/^PAPER_AGENT_CJK_SCALE=(0\.78|0\.86)$/PAPER_AGENT_CJK_SCALE=0.70/' "$CONFIG"
fi
if grep -Eqx 'PAPER_AGENT_IMAGE_QUALITY=medium' "$CONFIG"; then
  sed -Ei 's/^PAPER_AGENT_IMAGE_QUALITY=medium$/PAPER_AGENT_IMAGE_QUALITY=low/' "$CONFIG"
fi
printf '%s\n' "$RELEASE_VERSION" >"$VERSION_FILE"

rm -rf "$SETTINGS_APP.new"
cp -a "$INCOMING/settings-app" "$SETTINGS_APP.new"
rm -rf "$SETTINGS_APP"
mv "$SETTINGS_APP.new" "$SETTINGS_APP"

chmod 0755 "$NATIVE/paper-agent-native" "$NATIVE/native-oracle-service.sh" \
  "$NATIVE/native-selection-prepare.sh" "$NATIVE/native-selection-write.sh"
chmod 0755 "$IMAGE_PLUGIN" "$START_HOOK" "$SETTINGS_APP/backend" \
  "$SETTINGS_APP/backend/entry"
chmod 0644 "$NATIVE/"*.mjs "$NATIVE/"*.ts "$NATIVE/skills/"*/SKILL.md \
  "$ICONS/"*.svg "$UNIT_SOURCE" "$QMD_FILE" "$SETTINGS_APP/manifest.json" \
  "$SETTINGS_APP/resources.rcc" "$SETTINGS_APP/icon.png" "$VERSION_FILE"
chmod 0600 "$CONFIG"
mkdir -p "$NATIVE/jobs" "$NATIVE/oracle-data" "$NATIVE/artifacts"
chmod 0700 "$NATIVE/jobs" "$NATIVE/oracle-data" "$NATIVE/artifacts" "$BASE/selection"

if ! systemctl daemon-reload \
  || ! systemctl enable paper-agent-native-oracle.service >/dev/null \
  || ! systemctl restart paper-agent-native-oracle.service; then
  echo "Paper Agent install rolled back: oracle service failed" >&2
  exit 1
fi

ready=0
attempt=0
while [ "$attempt" -lt 30 ]; do
  if /home/root/node/bin/node "$NATIVE/native-oracle-client.mjs" --health \
    >/tmp/paper-agent-health.log 2>&1; then
    ready=1
    break
  fi
  attempt=$((attempt + 1))
  sleep 1
done
if [ "$ready" -ne 1 ] \
  || [ "$(stat -c %a /run/paper-agent-native-oracle.sock 2>/dev/null || true)" != 600 ]; then
  cat /tmp/paper-agent-health.log >&2 || true
  echo "Paper Agent install rolled back: oracle health check failed" >&2
  exit 1
fi

START=$(date +%s)
if ! /home/root/xovi/start; then
  echo "Paper Agent install rolled back: Xochitl failed" >&2
  exit 1
fi
sleep 12
LOG=$(journalctl -u xochitl --since "@$START" --no-pager -o cat)
if ! systemctl is-active --quiet xochitl \
  || printf '%s\n' "$LOG" | grep -Eq 'Error while processing file tree:.*paperAgentSelection\.qmd|paperAgentSelection\.qmd.*(Cannot locate element in tree|Error|failed)|Cannot assign to non-existent property "onPaperAgent|Application is quitting' \
  || printf '%s\n' "$LOG" | grep -Eq 'Could not find "?(file:///)?/home/root/paper-agent/assets/icons/paper-agent-(ai|beautify)\.svg|QML Image: Cannot open:.*paper-agent-(ai|beautify)\.svg|Error decoding.*paper-agent-(ai|beautify)\.svg' \
  || printf '%s\n' "$LOG" | grep -Eqi 'paper-agent-image.*(cannot load|undefined symbol|failed|error)' \
  || ! printf '%s\n' "$LOG" | grep -q '\[qmldiff\].*Loading file paperAgentSelection\.qmd'; then
  echo "Paper Agent install rolled back: QMD validation failed" >&2
  exit 1
fi

COMMITTED=1
echo "$BACKUP" >"$BASE/install-last-backup"
echo "paper_agent=installed"
echo "release_version=$RELEASE_VERSION"
cat /tmp/paper-agent-health.log
echo "settings_app=installed"
echo "xochitl=active"

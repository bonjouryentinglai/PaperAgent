#!/bin/sh
set -eu

# /etc is a volatile overlay on Paper Pro Move. Recreate the runtime unit every
# time XOVI starts; the source unit and this hook live on encrypted /home.
SOURCE=/home/root/paper-agent/systemd/paper-agent-native-oracle.service
TARGET=/run/systemd/system/paper-agent-native-oracle.service

test -f "$SOURCE"
mkdir -p /run/systemd/system
ln -sf "$SOURCE" "$TARGET"
systemctl daemon-reload
systemctl restart paper-agent-native-oracle.service

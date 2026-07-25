# Paper Agent Settings

This is a full AppLoad application with a QML frontend and a narrow backend.
The frontend edits only four Paper Agent settings. The backend forwards
allowlisted JSON to `settings-controller.mjs`; it cannot access ChatGPT OAuth
credentials or run arbitrary commands from QML.

Build an AppLoad-ready folder:

```sh
device/settings/build.sh dist/paper-agent-settings
```

The build needs Qt's `rcc`, Rust, and an `aarch64-unknown-linux-gnu` linker.
The resulting directory is installed as
`/home/root/xovi/exthome/appload/paper-agent-settings`.

The backend protocol adapter is derived from AppLoad's GPL-3.0-only Rust
client. It remains a separate GPL-3.0-only component; the QML and Paper Agent
settings controller are MIT licensed.

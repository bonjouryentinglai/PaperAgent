# Paper Agent Installer

Phase 2B uses a Wails v2 desktop shell with a Go backend. The first
implementation milestone provides:

- USB-first and LAN device discovery adapted from remagic;
- key or password SSH connection without requiring a local `ssh` binary;
- read-only device, developer-mode, dependency, login, and Paper Agent status;
- a confirmed Paper Agent-only uninstall that preserves shared dependencies,
  ChatGPT credentials, user configuration, runtime, and backups;
- a native macOS, Windows, and Linux UI foundation for the guided workflow.

Install, update, repair, and ChatGPT login mutations are deliberately not
enabled until checksum-verified release bundles and their transaction paths are
implemented and tested. The UI reports this clearly instead of exposing
partial operations.

Run Go tests:

```sh
cd installer
go test ./...
```

Build the desktop app with the stable Wails v2 CLI:

```sh
wails build
```

The device and probe code is adapted from remagic under the MIT license. See
`third_party/remagic-MIT.txt`.

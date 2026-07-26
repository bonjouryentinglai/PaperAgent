# Paper Agent Installer

Phase 2B is a Wails v2 desktop application with a Go backend. It provides one
guided workflow on macOS, Windows, and Linux:

1. Read the in-app Developer Mode preparation steps, sync or back up notebooks,
   and accept the unofficial-software disclaimer.
2. Connect and discover a Developer Mode Paper Pro Move.
3. Enter the device password once for the current installer session and run an
   allowlisted preflight. The password is never saved.
4. Verify model, OS, storage, XOVI/AppLoad, native dependencies, Node/Pi,
   ChatGPT login, Paper Agent, and the Settings app.
5. Install any missing prerequisites from pinned HTTPS artifacts whose byte
   length and SHA-256 are checked before upload.
6. Run Pi's OpenAI/Codex device-code login on the Move. The desktop displays
   only the approval URL and code; `auth.json` stays on the tablet.
7. Download a checksum-verified Paper Agent release and apply it with device
   backup, health checks, QMD verification, and automatic rollback.
8. Use the same verified transaction for Install, Update, or Repair.

Safe uninstall removes Paper Agent's executable integration and Settings app,
then signs out its `openai-codex` ChatGPT login. It preserves shared
XOVI/AppLoad components, Node/Pi, other Pi provider credentials, `config.env`,
the runtime, and backups. Temporary jobs and generated outputs inside the
removed native runtime are deleted. The confirmation is enforced in both the
UI and Go backend.

Existing XOVI and AppLoad installations are detected and reused. Paper Agent
does not claim ownership of those shared components and never removes them;
another reMarkable tool may depend on the same installation. Repair refreshes
Paper Agent and rebuilds its QML index, while an already detected
XOVI/AppLoad installation remains in place.

New installations record which prerequisites the installer actually creates.
When that record exists, the uninstall card offers an explicit full-removal
option for only those recorded components. This can include XOVI, AppLoad and
all of its apps, Node/Pi, XOVI persistence, settings, runtime, and backups.
Paper Agent's ChatGPT sign-in is already removed by the normal uninstall step;
other Pi provider credentials are preserved. Older or externally managed
installations without ownership evidence remain limited to safe uninstall.

## Security boundaries

- The installer repeats reMarkable's official Developer Mode paths and warns
  that enabling it factory-resets the device before any connection attempt.
- Install, Update, Repair, and Uninstall stay disabled until the user accepts
  the in-app unofficial-software and backup acknowledgment.
- Developer passwords exist only in memory until the installer closes.
- The installer never reads or copies OAuth access or refresh tokens.
- Published manifests and artifacts must use HTTPS.
- Release bundles and pinned dependencies are rejected on size or checksum
  mismatch.
- Only `chiappa` is accepted until other models pass physical validation.
- QMD and service activation failures trigger the device-side rollback.

The connection code and XOVI/AppLoad setup sequence are adapted from remagic
under its MIT license. See `third_party/remagic-MIT.txt`.

## Build and test

```sh
cd installer
go test ./...
go run github.com/wailsapp/wails/v2/cmd/wails@v2.13.0 build
```

`PAPER_AGENT_RELEASE_MANIFEST_URL` can point a maintainer build at an HTTPS or
loopback development manifest. Normal builds use the latest published GitHub
release.

`paper-agent-manifest.example.json` documents manifest schema 1. A relative
bundle URL is resolved beside the manifest. GitHub's release workflow builds
the ARM64 bundle and unsigned desktop binaries; a tagged build creates a draft
release for maintainer review. Platform code signing/notarization remains a
release-operations responsibility.

# Paper Agent Installer

Phase 2B is a Wails v2 desktop application with a Go backend. It provides one
guided workflow on macOS, Windows, and Linux:

1. Connect and discover a Developer Mode Paper Pro Move.
2. Enter the device password once for the current installer session and run an
   allowlisted preflight. The password is never saved.
3. Verify model, OS, storage, XOVI/AppLoad, native dependencies, Node/Pi,
   ChatGPT login, Paper Agent, and the Settings app.
4. Install any missing prerequisites from pinned HTTPS artifacts whose byte
   length and SHA-256 are checked before upload.
5. Run Pi's OpenAI/Codex device-code login on the Move. The desktop displays
   only the approval URL and code; `auth.json` stays on the tablet.
6. Download a checksum-verified Paper Agent release and apply it with device
   backup, health checks, QMD verification, and automatic rollback.
7. Use the same verified transaction for Install, Update, or Repair.

Uninstall removes only Paper Agent's executable integration and Settings app.
It preserves shared XOVI/AppLoad components, Node/Pi, ChatGPT credentials,
`config.env`, the runtime, and backups. Temporary jobs and generated outputs
inside the removed native runtime are deleted. The confirmation is enforced in
both the UI and Go backend.

Existing XOVI and AppLoad installations are detected and reused. Paper Agent
does not claim ownership of those shared components and never removes them;
another reMarkable tool may depend on the same installation. Repair refreshes
Paper Agent and its own native bridge, while an already detected XOVI/AppLoad
installation remains in place.

## Security boundaries

- Developer passwords exist only in memory until the installer closes or the
  user clicks **Forget password**.
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

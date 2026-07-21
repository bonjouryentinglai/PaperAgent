# Repository instructions

Paper Agent is developed from a local Codex checkout on a maintainer-controlled
host. GitHub is the source of truth for published commits, pull requests and
automated validation. A connected reMarkable is not required for ordinary
implementation, documentation, review or host-safe tests.

## Default workflow

1. Modify the local checkout with Codex.
2. Create a focused branch for each change.
3. Run `scripts/check.sh` before committing.
4. Keep the change reviewable and document user-visible behavior.
5. Push the branch and use GitHub CI as the off-device acceptance gate.
6. Clearly report any physical checks that remain pending.

Do not treat an unavailable Move as a blocker for work that can be validated
off-device. Do not claim that a device-affecting change works on hardware until
it has passed the relevant checklist in `docs/DEVELOPMENT.md`.

Do not assume Codex Cloud is the default execution environment. Mobile Remote
may steer a chat running on a connected Codex host, but files, commands,
credentials and tools still come from that host. Keep the host available when
remote control is required.

## Device boundary

Only run SSH, install, XOVI, build-on-Move or other device-mutating commands when
the user explicitly says a Move is connected and requests device work. The
device is normally reached from a Mac or Linux host on its private USB/network
address; hosted GitHub and Codex environments cannot reach it directly.

Changes to QMD integration, coordinates, pen restoration, native image
insertion, page creation, Undo/Redo, refresh behavior or sync require physical
acceptance on a Paper Pro Move. An ARM64 build alone is not physical acceptance.

## Safety and repository hygiene

- Never commit credentials, passwords, OAuth data, notebook captures, device
  logs, personal paths or local `config/paper-agent.env` files.
- Preserve upstream notices and the license boundaries in
  `THIRD_PARTY_NOTICES.md`.
- Keep QML thin; parsing, validation, model orchestration and rendering belong
  outside Xochitl.
- Treat every selection and every model response as untrusted input.
- Keep device changes reversible and preserve the original notebook content
  unless an undo-safe replacement path has been explicitly implemented.

See `docs/DEVELOPMENT.md` for local checks and the complete physical acceptance
checklist.

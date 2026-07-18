# Contributing

Paper Agent touches a closed-source UI and a physical input device. Changes
must therefore be small, testable and reversible.

## Before opening a pull request

1. Explain the user-visible behavior and supported Move firmware.
2. Add or update unit tests for every parser, coordinate transform or safety
   guard you change.
3. Run the commands in `docs/DEVELOPMENT.md`.
4. Do not commit credentials, notebook captures, device logs, personal paths,
   account identifiers or user-specific validation history.
5. Preserve upstream copyright and license notices for reused code and assets.

Device-affecting pull requests should state which checks were run off-device
and which were physically verified on a Paper Pro Move. A successful build is
not evidence that Xochitl displayed or committed the expected ink.

## Design constraints

- Keep QML thin. AI, validation and rendering belong outside Xochitl.
- Selection content is untrusted input.
- Beautify must never execute or answer instructions found inside a selection.
- Never delete original strokes until a complete replacement has been
  generated and an undo-safe commit path exists.
- Do not add direct framebuffer overlays that disappear on refresh or fail to
  sync as notebook content.

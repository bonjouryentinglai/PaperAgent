# Paper Agent

Paper Agent adds two AI actions directly to the native Xochitl selection menu
on a reMarkable Paper Pro Move:

- **AI** interprets the selected handwriting and can answer, calculate, create
  a formatted Markdown document, create a native table, or draw a bounded
  vector figure. It can also generate a raster image through GPT Image. One
  answer may mix text, tables, code and vector drawings.
- **Beautify** treats the selection as source material only. It preserves the
  original and writes a handwriting transcription or reconstructed native
  vector below it; it never answers the selected content.

Text, tables and vectors are written back through Xochitl's Marker input path,
so generated ink remains selectable, movable, resizable and undoable like
ordinary strokes. Raster output uses Xochitl's native notebook-image path.

> [!WARNING]
> Paper Agent is a developer preview for the Paper Pro Move (`chiappa`). It
> modifies the closed-source Xochitl UI through XOVI/QMD and may require updates
> after every reMarkable OS release. Back up important notebooks before testing.

## Current status

| Capability | Status |
|---|---|
| AI answers and calculations | Implemented; streams sentence-sized native ink |
| Rich and mixed answers | Implemented locally; headings, paragraphs, lists, bold, inline/fenced code, tables and vectors; device acceptance pending |
| Traditional Chinese handwriting | Implemented with ChenYuLuoyan and Open Huninn fallback |
| Native tables | Implemented in the bounded local renderer; device acceptance pending |
| Native vector figures | Bounded non-executable DSL with lines, polygons, sampled curves, rounded shapes, arcs and sparse hatch fills; device acceptance pending |
| Beautify text or sketch | Preserves the source and writes a complete validated text/vector result below it; physical acceptance pending |
| GPT raster images | Implemented locally: ChatGPT OAuth, `gpt-image-2`, bounded PNG normalization and Xochitl 3.27 native image insertion; physical acceptance pending |
| One-click install and settings UI | Planned |

Paper Agent is intentionally independent from any standalone notebook app. It
contains only the Xochitl integration, its local runtime and native renderer.

## How it works

```text
Xochitl lasso
  -> AI or Beautify QMD action
  -> selection PNG + explicit action
  -> local Pi RPC process using the user's ChatGPT login
  -> validated text, rich document, table, vector, or image result
  -> bounded StrokeJob -> guarded Marker writeback
     OR bounded PNG -> guarded Xochitl image insertion
```

OAuth credentials stay in Pi's mode-`0600` credential store on the tablet.
They are never placed in this repository, QML, command arguments, screenshots,
or the Paper Agent socket protocol.

## Developer installation

General-user installation is a productization milestone. The current workflow
is intended for contributors who already have Developer Mode, XOVI and AppLoad:

```sh
export PAPER_AGENT_HOST=remarkable.local

scripts/bootstrap-runtime.sh
scripts/login.sh
scripts/install-xovi-deps.sh
scripts/build-on-move.sh
scripts/install-device.sh dist/paper-agent-native
scripts/doctor.sh
```

`login.sh` opens Pi's interactive OpenAI/Codex device login. A ChatGPT
subscription is used through that provider; no OpenAI API key is required.
This route is not a public reMarkable or ChatGPT API and may change upstream.

See [Development](docs/DEVELOPMENT.md) for prerequisites and build details,
[Architecture](docs/ARCHITECTURE.md) for trust boundaries,
[Features](docs/FEATURES.md) for the exact capability matrix, and
[Roadmap](docs/ROADMAP.md) for the user-facing installer/settings plan.

## Safety

- The QMD passes an allowlisted action as a direct `systemd-run` argument; it
  never constructs a shell command from notebook content.
- The local oracle socket is owner-only and accepts only bounded selection PNGs
  under Paper Agent's runtime directory.
- Model output is parsed into non-executable result formats before rendering.
- Beautify never deletes or edits the lassoed source; it accepts only complete,
  validated text/vector output and uses the same guarded write-below path as AI.
- Generated images are decoded, dimension- and memory-bounded, normalized to
  RGBA PNG and inserted only into the page that originated the request.
- The native writer requires the Move hardware identity, active Xochitl, a
  single-writer lock and an explicit confirmation string.
- Install scripts preserve the previous files before replacement.

## Project policy

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and
[SECURITY.md](SECURITY.md) before opening a pull request or reporting a
vulnerability.

## License

Paper Agent's original Rust and JavaScript code is MIT licensed. The QMD file
is GPL-3.0-only because it contains code adapted from GPL-3.0 Xochitl
extensions. Fonts and dictionaries retain their own licenses. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the complete boundary.

Paper Agent is an independent community project and is not affiliated with or
endorsed by reMarkable or OpenAI.

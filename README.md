# Paper Agent

Paper Agent brings AI directly into the native selection menu on a reMarkable
Paper Pro Move. Lasso something in your notebook, then choose one of two actions:

**[Roadmap](ROADMAP.md)** · [Features](docs/FEATURES.md) ·
[Development](docs/DEVELOPMENT.md) · [Compatibility](docs/COMPATIBILITY.md)

<table>
  <tr>
    <td width="80" align="center">
      <img src="docs/assets/icons/paper-agent-ai.svg" alt="AI icon" width="56">
    </td>
    <td>
      <strong>AI</strong><br>
      Understands the selected handwriting and creates a useful answer in the
      notebook.
    </td>
  </tr>
  <tr>
    <td width="80" align="center">
      <img src="docs/assets/icons/paper-agent-beautify.svg" alt="Beautify icon" width="56">
    </td>
    <td>
      <strong>Beautify</strong><br>
      Cleans up the selected handwriting or sketch without changing its meaning.
    </td>
  </tr>
</table>

Text, tables, Sudoku grids, diagrams, and charts are described as semantic
Scenes and written back as native ink, so they remain selectable, movable,
resizable, and undoable. Scene geometry preserves proportions and can use the
normal Xochitl palette plus thin, medium, and thick lines. Generated pictures
are inserted as native notebook images. Long-running thinking and
image-generation requests can be cancelled from the on-device status bar.

> [!WARNING]
> Paper Agent is a developer preview for the Paper Pro Move (`chiappa`). It
> modifies the closed-source Xochitl UI through XOVI/QMD and may require updates
> after a reMarkable OS release. Back up important notebooks before testing.

## Features

<h3>
  <img src="docs/assets/icons/paper-agent-ai.svg" alt="AI icon" width="36">
  AI
</h3>

Select a handwritten request and tap **AI**. Paper Agent chooses an output that
fits the request; one answer can also combine text, lists, code, and tables.

Click any demo to open the larger 320px version.

| What you write | What Paper Agent creates | Demo |
|---|---|---|
| Ask a question, request an explanation, or solve a calculation | A concise handwritten answer or a structured explanation | <a href="docs/assets/demos/ai-question-answer.gif"><img src="docs/assets/demos/ai-question-answer.gif" alt="AI question and answer demo" width="240"></a> |
| Ask for notes, a summary, an outline, or formatted content | A document with headings, paragraphs, lists, emphasis, and code | <a href="docs/assets/demos/ai-document.gif"><img src="docs/assets/demos/ai-document.gif" alt="AI formatted document demo" width="240"></a> |
| Ask to organize information into rows and columns | A native table with wrapped, centered cells | <a href="docs/assets/demos/ai-table.gif"><img src="docs/assets/demos/ai-table.gif" alt="AI table demo" width="240"></a> |
| Ask for a photo, illustration, poster, painting, or other picture | A GPT-generated image inserted into the notebook | <a href="docs/assets/demos/ai-image.gif"><img src="docs/assets/demos/ai-image.gif" alt="AI image generation demo" width="240"></a> |

<h3>
  <img src="docs/assets/icons/paper-agent-beautify.svg" alt="Beautify icon" width="36">
  Beautify
</h3>

Select existing content and tap **Beautify**. Paper Agent treats the selection
as source material, never as a question or instruction. The original remains
untouched while the cleaned result is placed below it or on a new page.

| What you select | What Paper Agent creates | Demo |
|---|---|---|
| Rough handwriting, including Traditional Chinese and Latin text | A faithful, more legible handwriting transcription | <a href="docs/assets/demos/beautify-handwriting.gif"><img src="docs/assets/demos/beautify-handwriting.gif" alt="Beautify handwriting demo" width="240"></a> |
| A rough diagram, flowchart, or labeled sketch | A faithful vector reconstruction with aligned shapes, straight connectors, and consistent labels | <a href="docs/assets/demos/beautify-diagram.gif"><img src="docs/assets/demos/beautify-diagram.gif" alt="Beautify drawing demo" width="240"></a> |

## Install

Phase 2B adds a guided desktop installer for macOS, Windows, and Linux. A
published release provides the installer beside a checksum-verified ARM64
device bundle.

1. Enable Developer Mode and connect the Paper Pro Move over USB.
2. Open **Paper Agent Installer**, choose **Discover**, and enter the developer
   password shown by reMarkable.
3. Confirm the device changes and choose **Install**. Missing pinned
   XOVI/AppLoad, native, Node, and Pi prerequisites are installed and verified.
4. Choose **Start ChatGPT sign-in**, open the displayed URL, and enter its
   device code. OAuth credentials are written only to the Move.
5. Choose **Install** again. Paper Agent activates only after its service,
   Xochitl integration, and Settings app pass verification; otherwise the
   previous Paper Agent files are restored.

> **AppLoad not visible after installation or a restart?** Wake the Move, then
> quickly press the power button three times. This starts or restarts XOVI so
> AppLoad can appear again.

The same desktop tool provides **Update**, **Repair**, and a confirmed
Paper-Agent-only **Uninstall**. Open **Paper Agent Settings** in AppLoad to
change the model, thinking level, AI answer size, and minimum automatic scale.
Tagged builds are created as draft releases for maintainer review; platform
signing and fresh-device physical acceptance are still required before the
installer is called stable.

### Maintainer installation with Claude or Codex

Until a reviewed desktop build is published, maintainers can open this
repository in Claude Code or Codex and use:

```text
Install Paper Agent from this repository on my reMarkable Paper Pro Move.

Before changing the device:
1. Read README.md, docs/DEVELOPMENT.md, and docs/COMPATIBILITY.md.
2. Ask me for the device hostname or IP address and confirm that Developer Mode,
   XOVI, and AppLoad are available.
3. Check that my device model and software version are compatible. Stop and
   explain the problem if they are not.
4. Show me the installation plan and the rollback path.

Then use the repository's scripts to bootstrap the runtime, perform the
interactive ChatGPT login, install the pinned XOVI dependencies, build the
native components, install Paper Agent, and run scripts/doctor.sh.

Do not print, copy, or commit OAuth credentials, passwords, notebook content,
device logs, or private configuration. Ask before any destructive or
device-modifying action. When finished, report every check that passed, every
check that could not be run, and how to uninstall Paper Agent safely.
```

The ChatGPT login stays on the Move in Pi's private credential store. No OpenAI
API key is required. Contributors who want the individual commands and build
details can read [Development](docs/DEVELOPMENT.md).

### Data and authentication

- Paper Agent captures the selected notebook region as a PNG. The on-device Pi
  runtime sends that image, the selected action, and the associated prompt to
  OpenAI for processing. Do not select content that you do not want to send to
  an external AI service.
- ChatGPT OAuth credentials stay on the Move in
  `/home/root/.pi/agent/auth.json`, which must remain owner-readable only
  (`0600`). They must never be committed, copied into diagnostics, or shared in
  an issue.
- This developer preview uses Pi's ChatGPT/Codex subscription login and the
  Codex Responses transport at `chatgpt.com/backend-api/codex/responses`. That
  transport is not documented as a stable public OpenAI API for third-party
  applications and may stop working after an upstream change. The supported
  public OpenAI API uses API-key authentication; Paper Agent does not currently
  implement that provider.
- `scripts/uninstall.sh` removes Paper Agent and signs out its `openai-codex`
  credential. Other Pi provider credentials in the same store are preserved.
  Deleting `auth.json` manually would remove every Pi provider credential, so
  the uninstaller edits the store selectively instead.

See [Security](SECURITY.md) for the complete trust boundary and
[OpenAI's Codex authentication documentation](https://developers.openai.com/codex/auth)
for the officially documented Codex sign-in surfaces.

## Documentation

- [Features](docs/FEATURES.md) contains the detailed capability matrix.
- [Development](docs/DEVELOPMENT.md) contains prerequisites, build commands,
  and device validation steps.
- [Compatibility](docs/COMPATIBILITY.md) records the supported device and
  software assumptions.
- [Architecture](docs/ARCHITECTURE.md) explains the integration and trust
  boundaries.
- [Roadmap](ROADMAP.md) shows the current status and planned direction.

## Special thanks

Paper Agent exists because of the work shared by the reMarkable and open-source
communities. Special thanks to:

- [XOVI](https://github.com/asivery/xovi) and
  [rm-xovi-extensions](https://github.com/asivery/rm-xovi-extensions) for the
  extension runtime and ecosystem that make native Xochitl integration possible.
- [remarkable-doc-links](https://github.com/Marty-W/remarkable-doc-links) by
  Martin Weber for the selection-menu, selection-bound, DocumentView, and native
  image-paste foundations adapted by Paper Agent.
- [alefaraci/xovi-qmd-extensions](https://github.com/alefaraci/xovi-qmd-extensions)
  and
  [FouzR/xovi-extensions](https://github.com/FouzR/xovi-extensions), plus
  [StarNumber12046/xovi-qmd-extensions](https://github.com/StarNumber12046/xovi-qmd-extensions)
  for QMD techniques used in pen restoration and native page creation.
- [smart_remarkable](https://github.com/yangg1224/smart_remarkable) by Brock
  Wilcox for the native pen sequencing and skeleton-tracing foundations.
- [Pi](https://github.com/earendil-works/pi) by Mario Zechner for the agent
  runtime, ChatGPT authentication, and RPC transport used on the Move.
- [OpenClaw](https://github.com/openclaw/openclaw) for the image-generation
  request and event-extraction reference.
- [OpenCC](https://github.com/BYVoid/OpenCC), Kalam, ChenYuLuoyan, and Open
  Huninn for the language, handwriting, and font assets that make mixed Chinese
  and Latin output possible.

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for exact versions,
copyright notices, licenses, and the boundary between included and external
components.

## Contributing and security

These files are not required to use Paper Agent, but they document how a public
project should accept changes and private vulnerability reports:
[CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

Paper Agent's original Rust and JavaScript code is MIT licensed. The QMD and
the separate native XOVI image plugin are GPL-3.0-only because they contain
code adapted from GPL-3.0 Xochitl extensions. Fonts and dictionaries retain
their own licenses. See
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the complete boundary.

Paper Agent is an independent community project and is not affiliated with or
endorsed by reMarkable or OpenAI.

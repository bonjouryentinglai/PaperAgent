# Development

## Local checks

```sh
scripts/check.sh
```

The native crate can be tested on ordinary Linux because hardware access is
only performed by explicit commands. A real ARM64 Move build is still required
before installation.

GitHub Actions runs the same off-device checks for pull requests and pushes.
Passing those checks is not evidence of physical behavior; use the acceptance
checklist below when a change is ready to test on a connected Move.

## Reproducible image-plugin build

`scripts/build-image-plugin.sh` cross-builds the GPL image bridge with two
immutable default inputs:

- `eeems/remarkable-toolchain@sha256:297237d78a2aafa14896dd6a1495f7af173bcd646b0359be72ac00c04483dda3`
- XOVI commit `2b99649f5e4fd6288be7792a8570bd16418adb70`

If `/tmp/paper-agent-xovi` exists, the script verifies that it is clean and at
the pinned commit. Otherwise it fetches exactly that commit inside the
container. Maintainers can deliberately test replacements with
`PAPER_AGENT_TOOLCHAIN_IMAGE`, `PAPER_AGENT_XOVI_COMMIT`, and
`PAPER_AGENT_XOVI_SOURCE`; release builds should use the recorded defaults.

## Move build

`scripts/build-on-move.sh` uploads only `device/native`, builds it in a
temporary directory with an existing Rust toolchain and downloads the resulting
binary to `dist/`. It does not install or execute that binary.

Set these when your paths differ:

```sh
export PAPER_AGENT_HOST=remarkable.local
export PAPER_AGENT_DEVICE_USER=root
export PAPER_AGENT_CARGO_HOME=/home/root/paper-agent/build-toolchain/cargo
export PAPER_AGENT_RUSTUP_HOME=/home/root/paper-agent/build-toolchain/rustup
```

## Physical acceptance

After an install, verify separately:

1. AI text in English, Traditional Chinese and mixed text.
2. A calculation with a short result.
3. A two- and four-column table at several selection positions; confirm every
   cell is horizontally and vertically centered.
4. Lines, arrows, open/closed polylines, quadratic/cubic curves, rounded
   rectangles, arcs, dots and hatch-filled polygon/ellipse/pie shapes.
   For semantic Scene acceptance, draw a large circular or elliptical arc plus
   quadratic and cubic paths; confirm they remain smooth at normal viewing
   distance, preserve their intended proportions and join adjacent lines
   without visible gaps.
5. Phase 2A Scene: a 9x9 Sudoku grid must have square cells, stronger 3x3
   boundaries and centered digits; repeat in both a portrait and a wide output
   box and confirm circles/squares keep their proportions.
6. Phase 2A styles: exercise black, gray, blue, red, green, yellow, cyan and
   magenta plus thin, medium and thick lines. Confirm the user's original pen
   palette and thickness are restored after success, failure and cancellation.
7. Phase 2A alignment and text: compare left, centered and right labels; repeat
   Chinese and Latin glyphs at equal sizes and confirm Beautify keeps one
   output line per source line.
8. One mixed answer containing a heading, list, bold and inline code, fenced
   code, a table and a vector block in the expected order.
9. Beautify text does not answer a question embedded in the selection; it
   preserves the selected source and writes the transcription below it at
   approximately the lasso's original width, height and visual scale.
10. Beautify a deliberately rough circle/box/arrow mind map. Confirm the result
   preserves labels and connections while making circles round, boxes level,
   connectors straight and repeated nodes consistent.
11. Test an offline or forced-error Beautify request and confirm the source
   remains unchanged.
12. Test Undo/Redo after both Beautify routes.
13. Thinking, writing, success, cancellation and error notifications. Start a
    slow request, press Cancel while it is thinking, then confirm no ink is
    written and the next request starts immediately.
14. A GPT photo or ordinary illustration appears as a native image object; move
   and resize it, then test Undo/Redo, close/reopen, reboot, export and sync.
15. Start a GPT image request and change pages before it finishes; Paper Agent
    must reject the stale insertion and remove its temporary artifact.
    Also request a portrait image near the page bottom and confirm it is placed
    at the top of a newly created page instead of covering the selection.
16. AI adaptive layout: verify a short answer remains at 100%, a longer answer
    fits between 60% and 99%, and a result that cannot fit at 60% creates a new
    native page that repeats the same 100%-to-60% fit policy. If it still needs
    multiple pages at 60%, confirm every page keeps the same 60% scale.
17. Beautify a lasso near the page bottom whose same-size result cannot fit;
    confirm Xochitl creates a new page and preserves the lasso-based visual size.
18. Undo, move, resize, page save, reopen and sync behavior for native ink.
19. Restart Xochitl during selection capture or before writeback. The old
    request must stop, `scripts/doctor.sh` must report `coordinator_lock=clear`,
    and the next AI/Beautify request must start normally.
20. Open Paper Agent Settings from AppLoad. Confirm it loads the active model,
    thinking level, AI answer size, and minimum auto-scale; Exit must return to
    AppLoad without stopping the Paper Agent service. Confirm the service status
    changes between Ready, Busy, and Unavailable as appropriate. Stop the oracle
    service, press Restart service, and confirm the app reports Restarting and
    returns to Ready without restarting Xochitl or changing settings.
21. Change all four settings but press Exit before Apply. Verify Apply & Exit,
    Discard, and Cancel independently, including AppLoad's top-edge close
    gesture as a fallback.
22. Apply settings while Paper Agent is idle and verify only
    `paper-agent-native-oracle.service` restarts. The next AI answer must report
    the selected model/thinking and use the selected text scale. Start a slow AI
    request and confirm Apply is unavailable until that request finishes.
23. Force an invalid model activation in a maintainer test. The Settings app
    must report failure, restore the previous mode-0600 config, restart the old
    service successfully, and leave OAuth data untouched.
24. On a clean Developer Mode Move, run the desktop installer with only the
    USB cable and device password. Before connecting, confirm the installer
    shows the official on-device path for enabling Developer Mode and finding
    the generated SSH password, explains the factory reset, and presents the
    unofficial-software disclaimer. Confirm install and removal actions remain
    disabled until the acknowledgment is selected. Then confirm discovery, the
    `chiappa` model gate, storage check, pinned XOVI/AppLoad/native dependency
    install, Node/Pi install, and post-restart SSH recovery. After installation
    and again after a reboot, hide or stop XOVI, quickly press the power button
    three times, and confirm XOVI restarts and AppLoad becomes available.
25. Start ChatGPT sign-in in the installer. Confirm the URL and device code
    appear, cancellation leaves no partial credential, successful approval
    produces a mode-0600 `/home/root/.pi/agent/auth.json`, and no token appears
    in desktop logs, process output, or the repository.
26. Install a checksum-verified release. Confirm the reported version, active
    oracle service, Xochitl selection buttons, native image bridge, and
    Settings app. Corrupt a maintainer-only test bundle after its manifest is
    created and confirm the desktop rejects it before upload.
27. Update to a newer test version, then run Repair at the same version. Force
    service-health and QMD-validation failures separately; each must restore
    the previous working version and leave notebook content and OAuth data
    unchanged.
28. Use desktop Uninstall. Confirm Paper Agent, its Xochitl selection entry,
    image bridge, service, Settings app, and `openai-codex` ChatGPT login are
    gone while XOVI/AppLoad, Node/Pi, other Pi provider credentials,
    `config.env`, the runtime, and backups remain. Temporary jobs and generated
    outputs should be gone. Reinstall must require ChatGPT sign-in again.
29. On a clean installer-owned stack, confirm Full removal lists only recorded
    components, warns that installer-owned AppLoad apps will also be removed,
    and removes the recorded XOVI/AppLoad, persistence, Node/Pi, unchanged
    configuration, runtime, and backups. Repeat with
    pre-existing XOVI/AppLoad and confirm full removal is not offered for those
    components. Confirm other Pi provider credentials remain after Paper Agent
    signs out its own `openai-codex` login.

Vector acceptance should also confirm that hatch density remains readable at
small and large placements. Legacy vector results keep the active Xochitl ink
style. Phase 2A Scene objects can request supported palette colors and three
bounded pen widths; dash state is not implemented.

Image generation can take substantially longer than text even with Pi thinking
set to `off`. The default GPT Image quality is `low` to prioritize latency; it
can be changed in `config.env` for comparisons.

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
5. One mixed answer containing a heading, list, bold and inline code, fenced
   code, a table and a vector block in the expected order.
6. Beautify text does not answer a question embedded in the selection; it
   preserves the selected source and writes the transcription below it at
   approximately the lasso's original width, height and visual scale.
7. Beautify a deliberately rough circle/box/arrow mind map. Confirm the result
   preserves labels and connections while making circles round, boxes level,
   connectors straight and repeated nodes consistent.
8. Test an offline or forced-error Beautify request and confirm the source
   remains unchanged.
9. Test Undo/Redo after both Beautify routes.
10. Thinking, writing, success, cancellation and error notifications. Start a
    slow request, press Cancel while it is thinking, then confirm no ink is
    written and the next request starts immediately.
11. A GPT photo or ordinary illustration appears as a native image object; move
   and resize it, then test Undo/Redo, close/reopen, reboot, export and sync.
12. Start a GPT image request and change pages before it finishes; Paper Agent
    must reject the stale insertion and remove its temporary artifact.
    Also request a portrait image near the page bottom and confirm it is placed
    at the top of a newly created page instead of covering the selection.
13. AI adaptive layout: verify a short answer remains at 100%, a longer answer
    fits between 60% and 99%, and a result that cannot fit at 60% creates a new
    native page and returns to 100% there.
14. Beautify a lasso near the page bottom whose same-size result cannot fit;
    confirm Xochitl creates a new page and preserves the lasso-based visual size.
15. Undo, move, resize, page save, reopen and sync behavior for native ink.
16. Restart Xochitl during selection capture or before writeback. The old
    request must stop, `scripts/doctor.sh` must report `coordinator_lock=clear`,
    and the next AI/Beautify request must start normally.

Vector acceptance should also confirm that hatch density remains readable at
small and large placements. Paper Agent deliberately keeps the active Xochitl
ink color; per-command color, stroke width and dash state are not part of the
current native vector format.

Image generation can take substantially longer than text even with Pi thinking
set to `off`. The default GPT Image quality is `low` to prioritize latency; it
can be changed in `config.env` for comparisons.

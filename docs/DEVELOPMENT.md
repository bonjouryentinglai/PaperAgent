# Development

## Local checks

```sh
cd device/native
cargo fmt -- --check
cargo test --locked

cd ../..
node --check device/runtime/native-oracle-server.mjs
node device/runtime/native-oracle-server.mjs --self-test
node --check device/runtime/native-oracle-client.mjs
node --check device/runtime/rich-document.mjs
node --test device/runtime/rich-document.test.mjs
node --test device/runtime/image-generate.test.mjs

sh -n device/runtime/native-oracle-service.sh
sh -n device/runtime/native-selection-prepare.sh
sh -n device/runtime/native-selection-write.sh
```

The native crate can be tested on ordinary Linux because hardware access is
only performed by explicit commands. A real ARM64 Move build is still required
before installation.

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
3. A two- and four-column table at several selection positions.
4. Lines, arrows, open/closed polylines, quadratic/cubic curves, rounded
   rectangles, arcs, dots and hatch-filled polygon/ellipse/pie shapes.
5. One mixed answer containing a heading, list, bold and inline code, fenced
   code, a table and a vector block in the expected order.
6. Beautify text does not answer a question embedded in the selection.
7. Beautify drawing creates a faithful movable copy and preserves the source.
8. Thinking, success and error notifications.
9. A GPT photo or ordinary illustration appears as a native image object; move
   and resize it, then test Undo/Redo, close/reopen, reboot, export and sync.
10. Start a GPT image request and change pages before it finishes; Paper Agent
    must reject the stale insertion and remove its temporary artifact.
11. Undo, move, resize, page save, reopen and sync behavior for native ink.

Vector acceptance should also confirm that hatch density remains readable at
small and large placements. Paper Agent deliberately keeps the active Xochitl
ink color; per-command color, stroke width and dash state are not part of the
current native vector format.

Image generation can take substantially longer than text even with Pi thinking
set to `off`; that setting does not reduce the selected GPT Image quality.

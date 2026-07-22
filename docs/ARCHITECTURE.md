# Architecture

Paper Agent uses a thin Xochitl integration and a separate local runtime.

## Request path

1. The QMD adds AI and Beautify icons to Xochitl's lasso menu.
2. It snapshots selection bounds, asks `rm-shot` for a bounded PNG, then closes
   the lasso and restores the primary pen. `rm-shot` only acknowledges that its
   detached capture started, so the worker waits for a stable PNG and aborts if
   the Xochitl process that accepted the request is replaced.
3. The worker waits for a complete PNG and sends an allowlisted request to a
   mode-`0600` Unix socket.
4. A persistent Pi RPC process sees the image and finishes the turn with one of
   two terminating tools: `move_render_scene` or `move_generate_image`. Pi's
   built-in filesystem, shell and editing tools remain disabled, and a
   terminating tool does not require a second model round trip.
5. The oracle validates semantic Scene v1 JSON, preserves its proportions, and
   compiles it into bounded, non-executable native-vector runs grouped by pen
   style. Image requests use the narrow image helper, then decode and normalize
   the result to a bounded RGBA PNG.
6. Scene content is fitted as one proportion-preserving composition. The legacy
   marker path still preflights text, documents and tables from 100% down to
   60%, then asks Xochitl for a real page if necessary. Beautify never shrinks
   merely because the old page is short: its lasso-sized destination moves to
   a new page when required.
7. StrokeJobs use the guarded Marker writer, bound to the Xochitl PID that
   originated the request. Images use Xochitl 3.27's native
   scene-image insertion method only if the originating controller and page are
   still active. The image center is anchored one half-height below the lasso;
   its current-page drop point is not clamped against Chiappa's differently
   scaled `paperNoteBounds`. Beautify validates its entire Scene, preserves the
   source and writes the result below it through the same Marker path as AI.

The native status bar exposes Cancel while the request is still in model or
image generation. The local cancel client aborts the active Pi turn, terminates
the image child process and releases the coordinator before reporting the
cancelled lifecycle state. Once native writing begins, the control disappears
so cancellation cannot deliberately leave half an answer on the page.

## Phase 2A skills and tools

The trusted runtime loads three fixed policy skills from source files:
`ai-selection`, `structured-drawing`, and `beautify-selection`. They describe
identity, mode boundaries, faithful Beautify behavior, and generic layout
principles. They do not contain one prompt branch per game or diagram. For
example, Sudoku, a table and a monthly calendar all use the same semantic
`grid` object.

The model can call only:

| Tool | Purpose |
|---|---|
| `move_render_scene` | Text, calculations, grids, tables, diagrams, charts and line art as native ink |
| `move_generate_image` | Photos, paintings and other pixel imagery as a native image object |

Scene v1 supports text, lines, arrows, rectangles, circles, ellipses,
polylines and grids. Objects carry bounded coordinates, supported palette
colors, thin/medium/thick stroke weights, alignment and an optional logical
group. A Scene canvas has an explicit aspect ratio. The local contain mapping
letterboxes it into the available notebook destination, so a circle remains a
circle and a square remains a square in both portrait and landscape boxes.
Regular grids are expanded locally with separate major and minor weights; GPT
does not need to draw each Sudoku line independently.

## Legacy result protocol

The previous marker protocol remains parser-compatible while Phase 2A is
validated and deployed, but the Phase 2A prompt requires one terminating tool
call rather than marker text.

A legacy response begins with one marker line:

| Marker | Local interpretation |
|---|---|
| `::text` | Handwriting rasterization, thinning and native stroke tracing |
| `::document` | Bounded Markdown subset compiled into ordered rich, table and vector blocks |
| `::table` | Deterministic pipe-row parsing, grid layout and native strokes |
| `::vector` | Restricted `paper-agent-vector 1` DSL, bounded curves/hatch fills and native strokes |
| `::image` | Concise prompt sent to `gpt-image-2`, then a bounded PNG inserted as a native Xochitl image item |

All result kinds are buffered until complete so the renderer can preflight the
whole answer and no partial result is drawn before a possible page switch. A
document may contain headings, paragraphs, ordered or unordered lists, bold,
inline code, fenced code, pipe tables and fenced `paper-agent-vector` blocks.
Node validates its block structure and compiles it to a versioned manifest with
allowlisted kinds and hex-encoded UTF-8; Rust parses and bounds that manifest a
second time before producing native strokes. Code fences are displayed as
data and are never executed. Syntax highlighting is not part of the initial
native-ink renderer; code is distinguished with a framed, smaller style.

The legacy vector language supports lines, arrows, open and closed polylines,
quadratic/cubic sampled curves, rectangles, rounded rectangles, circles,
ellipses, arcs, dots and labels. `fillpoly`, `box`, `disc`, `fillellipse` and
`wedge` approximate filled areas with sparse hatch strokes in the user's active
ink color; they are not solid-color framebuffer fills. Node and Rust both
enforce coordinate, angle, command and path limits. Rust additionally caps one
vector result at 512 strokes, 24,000 source points and 400,000 planned path
points before Marker writeback.

An image result is available only through the resident service. The image
helper uses the same ChatGPT OAuth credential as Pi, requests `gpt-image-2`,
validates the returned PNG, and never receives general agent tools. The native
binary decodes it again, removes metadata, converts it to RGBA8 and scales it
down without upscaling. The QMD remembers the source SceneController and page;
it rejects and deletes a result if the user changed pages while generation was
running. Page fit is decided against the remaining 954x1696 framebuffer area
below the lasso, not Xochitl's differently scaled scene bounds. An image that
does not fit there moves to a new page and is placed at that page's top centre.
The source PNG is removed five seconds after acknowledged insertion.
Failures and cancellations delete it immediately, and an hourly sweep removes
any orphaned artifact older than 24 hours.

Beautify accepts only `::text` or `::vector`; untyped content and attempts to
return a table or image are rejected. It never streams partial output. The QMD
closes the lasso without deleting it, then creates a destination matching the
lasso width and height. That box is placed below when it fits; otherwise a QMD
bridge calls Xochitl's native `addPage`, waits for the document page count and
current page to change, and acknowledges the new page to the oracle. Text uses
the fit-to-box renderer; vectors map to the same target. The Beautify prompt
requires rough circles, boxes, connectors and arrows to be reconstructed with
clean aligned primitives while preserving labels and topology. Failures leave
the source untouched and do not require delete acknowledgement or rollback.

This deliberately does not edit notebook files, delete selected items or sweep
the rectangle with a virtual eraser.

## Trust boundaries

- **QML:** capture geometry and choose an action; no credentials or AI logic.
- **Pi:** owns ChatGPT authentication and model transport; only the two
  explicitly loaded terminating tools are enabled.
- **Scene compiler:** validates semantic JSON, preserves aspect ratio, batches
  style runs and emits only the existing bounded vector format.
- **Oracle socket:** local, owner-only, single active request, bounded PNG path.
- **Renderer:** parses only local data formats; no scripts, HTML or arbitrary SVG.
- **Image preparation:** rejects unsafe PNGs and publishes only bounded,
  owner-readable RGBA artifacts in an owner-only directory.
- **Writer:** validates hardware, axes, job size, lock, originating Xochitl PID
  and confirmation string. Shell-to-XOVI signals use non-blocking FIFO writes.

The Move mounts `/etc` as a volatile overlay. Paper Agent therefore stores its
systemd source unit under `/home/root/paper-agent/systemd` and installs an XOVI
post-start hook that recreates and starts the runtime unit whenever XOVI starts.

## Native image acceptance

Rendering a PNG directly over Xochitl's framebuffer would be temporary and
would not sync, move or undo as notebook content, so Paper Agent does not use
that approach. The local implementation instead calls Xochitl 3.27's native
scene-image insertion path with a local file URL and a scene-space center point.

This path is still a developer preview. It is not considered accepted until a
physical Move test verifies insertion, move, resize, Undo/Redo, close/reopen,
reboot, export and sync. The oracle retries the bounded image signal until QML
creates a positive or negative acknowledgement file; transport completion can
therefore no longer hide a dropped or rejected Xochitl insertion command.

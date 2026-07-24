# Roadmap

Paper Agent is growing from a native AI selection assistant into a safe agent
for the reMarkable workspace. The roadmap is directional and has no fixed
release dates.

## Current: developer preview

- [x] AI and Beautify actions in the native Xochitl selection menu.
- [x] ChatGPT subscription login without an OpenAI API key.
- [x] Native text, tables, structured documents, and vector drawings.
- [x] GPT-generated pictures inserted as notebook images.
- [x] Traditional Chinese and Latin handwriting output.
- [x] Progress, cancellation, adaptive multi-page text, install, and rollback
  support.
- [ ] Broader device, software, and physical acceptance coverage.

## Phase 2A: drawing foundation — complete for developer preview

- [x] Introduce the first Paper Agent skills and two bounded output tools.
- [x] Add a semantic Scene format for text, grids, diagrams, and charts.
- [x] Preserve proportions for grids, circles, squares, and diagrams.
- [x] Add varied line weights, supported colors, alignment, and logical-group
  metadata for future native grouping.
- [x] Keep native ink as the default, with native images when appropriate.
- [ ] Continue release hardening on Move for palette colors, pen restoration,
  lifecycle behavior, export, and sync.

## Phase 2A.1: current-page closed loop — deferred

- [x] Add and physically validate smooth semantic arcs and Bezier curves.
- [ ] Plan and execute multiple tool outputs, such as native ink plus a GPT
  image, in one request.
- [ ] Verify and revise content after it is written to the page.
- [ ] Treat one Paper Agent result as one reversible operation.
- [ ] Support confirmed, undo-safe replacement of original content.

Phase 2A already covers most everyday selection-assistant use. This long-tail
follow-up is intentionally deferred.

## Phase 2B: easy installation and onboarding

- [ ] Ship one guided setup tool for macOS, Windows, and Linux.
- [ ] Discover a connected Move and check developer mode, model, OS, storage,
  and required dependencies before making changes.
- [ ] Reuse the remagic installation stack for SSH, XOVI, and AppLoad instead
  of rebuilding those foundations.
- [ ] Download a checksum-verified Paper Agent release and install it with
  automatic backup and rollback.
- [ ] Guide ChatGPT subscription login while keeping OAuth credentials only on
  the Move.
- [ ] Verify the installation, then offer repair, update, and uninstall paths.

## Phase 3: multi-page creation

- [x] Continue long text results across pages.
- [ ] Create and navigate multiple pages for one task.
- [ ] Apply suitable page templates and layouts.
- [ ] Build calendars, planners, study packs, meeting notes, and workbooks.
- [ ] Preview large plans before creating many pages.

## Phase 4: notebook and library actions

- [ ] Create and name notebooks and folders.
- [ ] Open, rename, move, tag, and organize documents.
- [ ] Navigate to a requested notebook or page.
- [ ] Reorder, duplicate, or remove pages safely.
- [ ] Require confirmation for important or destructive changes.

## Phase 5: personal knowledge agent

- [ ] Search handwritten and imported notes.
- [ ] Answer with notebook and page references.
- [ ] Summarize notebooks and connect related pages.
- [ ] Find decisions, open questions, and unfinished tasks.
- [ ] Build optional review, learning, and meeting workflows with clear privacy
  boundaries.

The immediate contribution focus is **Phase 2B: easy installation and
onboarding**. Phase 2A release hardening can continue in parallel. See
[CONTRIBUTING.md](CONTRIBUTING.md) and [DEVELOPMENT.md](docs/DEVELOPMENT.md)
before working on device integration.

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
- [x] Progress, cancellation, new-page fallback, install, and rollback support.
- [ ] Broader device, software, and physical acceptance coverage.

## Phase 2A: drawing foundation — in progress

- [x] Introduce the first Paper Agent skills and two bounded output tools.
- [x] Add a semantic Scene format for text, grids, diagrams, and charts.
- [x] Preserve proportions for grids, circles, squares, and diagrams.
- [x] Add varied line weights, supported colors, alignment, and logical-group
  metadata for future native grouping.
- [x] Keep native ink as the default, with native images when appropriate.
- [ ] Complete physical Move acceptance for text quality, palette colors, pen
  widths, placement, Undo/Redo, reopen, export, and sync.

## Phase 2B: current-page agent

- [ ] Understand the selected content and available page space.
- [ ] Choose the output type and placement automatically.
- [ ] Combine text, tables, diagrams, and images in one response.
- [ ] Verify, revise, or undo content created by Paper Agent.
- [ ] Preserve original content unless a confirmed reversible action says
  otherwise.

## Phase 3: multi-page creation

- [ ] Continue long results across pages.
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

The immediate contribution focus is **Phase 2A**. See
[CONTRIBUTING.md](CONTRIBUTING.md) and [DEVELOPMENT.md](docs/DEVELOPMENT.md)
before working on device integration.

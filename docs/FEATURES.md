# Feature matrix

Paper Agent separates output capabilities from standalone notebook-app UI.
The public project owns only the native Xochitl selection workflow.

## AI action

| Capability | Local implementation | Physical Move status |
|---|---|---|
| Plain answers and calculations | Semantic Scene text with bounded single-line labels and native ink | Phase 2A physical acceptance pending |
| Formatted text | Headings, paragraphs, ordered/unordered lists, bold and inline code with the same body-size policy as plain text | Pending combined-build acceptance |
| Code blocks | Preserved verbatim, wrapped and drawn in a smaller framed style; never executed | Pending combined-build acceptance |
| Tables and regular grids | One semantic grid object with local cell layout, centered labels, major/minor line weights and bounded native ink | Phase 2A physical acceptance pending |
| Scene drawings | Lines, arrows, polylines, rectangles, circles, ellipses, text and grids with contain-fit proportions, supported colors, three line weights, alignment and logical groups | Phase 2A physical acceptance pending |
| GPT raster images | ChatGPT OAuth to `gpt-image-2` at the low-latency `low` quality setting, strict PNG validation, bounded RGBA normalization and guarded Xochitl 3.27 image insertion; images move to a new page rather than covering the source when they do not fit below | End-to-end physical acceptance pending |
| Mixed documents | Text, code, tables and vector blocks keep their original order in one result | Pending combined-build acceptance |

The image router sends photographs, photorealistic work, watercolor, paintings,
posters and other pixel-oriented artwork to GPT Image. Text, tables, Sudoku,
calendars, diagrams, charts, maps, schematics and geometric drawings use the
semantic Scene renderer instead.
Thinking and GPT Image generation can be cancelled before native writeback
starts. Cancellation aborts Pi, stops the image helper and releases all request
locks and temporary files.

## Beautify action

Beautify treats selected notebook content as untrusted source material rather
than an instruction:

- Text is transcribed into the configured handwriting face.
- A sketch is reconstructed with normalized circles, boxes, straight lines,
  consistent arrows and aligned repeated nodes through semantic Scene objects.
- Questions are not answered.
- GPT images are rejected in this mode.

The developer preview preserves the selected source. It buffers the complete
result, accepts only a bounded Scene tool call and writes it below the
lasso through the same guarded Marker path as AI. Its destination always keeps
the lasso width and height. It is written below when that whole box fits; if it
does not, Xochitl creates a new page and the box is rendered there without
shrinking for the old page's remaining space. Text uses a wider fit-to-box size
range so the transcription approximately fills that box. No
delete acknowledgement or rollback is required, and every failure leaves the
source unchanged.

Physical Move acceptance is still required for text/sketch placement and
Undo/Redo behavior.

## Deliberate differences

Phase 2A Scene supports the normal Xochitl palette and thin, medium and thick
native-pen styles, restoring the user's original pen afterward. Custom RGB,
syntax coloring, dashed strokes, smooth solid fills and pagination of one answer
across multiple new pages are not implemented. The current policy can move one
complete result to one new page, and filled vector areas use sparse native-ink
hatching.

An independent canvas, pen/eraser tools, chat drawer, page scrubber, app-level
power handling and full-screen takeover are standalone-application concerns;
they are intentionally outside Paper Agent.

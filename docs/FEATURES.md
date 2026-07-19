# Feature matrix

Paper Agent separates output capabilities from standalone notebook-app UI.
The public project owns only the native Xochitl selection workflow.

## AI action

| Capability | Local implementation | Physical Move status |
|---|---|---|
| Plain answers and calculations | Complete-answer preflight at 100%, adaptive fit down to a 60% floor, then native new-page fallback at 100% | Previously proven for the text path; adaptive layout acceptance pending |
| Formatted text | Headings, paragraphs, ordered/unordered lists, bold and inline code with the same body-size policy as plain text | Pending combined-build acceptance |
| Code blocks | Preserved verbatim, wrapped and drawn in a smaller framed style; never executed | Pending combined-build acceptance |
| Tables | Pipe-table parser, wrapped and centered cells, bounded native grid and shared body-size policy | Pending combined-build acceptance |
| Vector drawings | Lines, arrows, polylines, polygons, quadratic/cubic curves, rectangles, rounded rectangles, circles, ellipses, arcs, dots, labels and sparse hatch fills | Pending combined-build acceptance |
| GPT raster images | ChatGPT OAuth to `gpt-image-2`, strict PNG validation, bounded RGBA normalization and guarded Xochitl 3.27 image insertion | End-to-end physical acceptance pending |
| Mixed documents | Text, code, tables and vector blocks keep their original order in one result | Pending combined-build acceptance |

The image router deliberately sends photographs, photorealistic work,
watercolor, paintings, posters, ordinary illustrations and ambiguous requests
to "draw a picture" to GPT Image. Explicit diagrams, charts, maps, schematics,
line drawings and vector requests use the safe vector renderer instead.

## Beautify action

Beautify treats selected notebook content as untrusted source material rather
than an instruction:

- Text is transcribed into the configured handwriting face.
- A sketch is reconstructed with normalized circles, boxes, straight lines,
  consistent arrows and aligned repeated nodes through the safe vector renderer.
- Questions are not answered.
- Documents, tables and GPT images are rejected in this mode.

The developer preview preserves the selected source. It buffers the complete
result, accepts only text/vector output and writes a bounded result below the
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

The native writer currently uses the active Xochitl ink color. Per-command
color, syntax coloring, variable stroke width, dashed strokes, smooth solid
fills and pagination of one answer across multiple new pages are not implemented.
The current policy can move one complete result to one new page. Code uses a
framed monochrome style, and filled vector areas use sparse native-ink hatching.

An independent canvas, pen/eraser tools, chat drawer, page scrubber, app-level
power handling and full-screen takeover are standalone-application concerns;
they are intentionally outside Paper Agent.

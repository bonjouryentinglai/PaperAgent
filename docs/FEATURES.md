# Feature matrix

Paper Agent separates output capabilities from standalone notebook-app UI.
The public project owns only the native Xochitl selection workflow.

## AI action

| Capability | Local implementation | Physical Move status |
|---|---|---|
| Plain answers and calculations | Sentence-sized streaming to native handwriting strokes | Previously proven for the text path; re-test after the split |
| Formatted text | Headings, paragraphs, ordered/unordered lists, bold and inline code | Pending combined-build acceptance |
| Code blocks | Preserved verbatim, wrapped and drawn in a smaller framed style; never executed | Pending combined-build acceptance |
| Tables | Pipe-table parser, wrapped cells, bounded native grid and text strokes | Pending combined-build acceptance |
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
- A sketch is reconstructed with the safe vector renderer.
- Questions are not answered.
- Documents, tables and GPT images are rejected in this mode.

The developer preview replaces the selected source in place. It buffers the
complete result, accepts only text/vector output, renders and dry-runs a bounded
stroke job, opens the native writer, then asks Xochitl to delete the still-live
selection. Writeback starts only after Xochitl acknowledges that delete. Page or
selection changes and every failure before acknowledgement preserve the source.

Physical Move acceptance is still required for text/sketch placement, failure
recovery and Undo/Redo behavior. Single-step Undo grouping across Xochitl's
native delete and virtual-Marker write remains a stability milestone.

## Deliberate differences

The native writer currently uses the active Xochitl ink color. Per-command
color, syntax coloring, variable stroke width, dashed strokes, smooth solid
fills and automatic multi-page continuation are not implemented. Code uses a
framed monochrome style, and filled vector areas use sparse native-ink hatching.

An independent canvas, pen/eraser tools, chat drawer, page scrubber, app-level
power handling and full-screen takeover are standalone-application concerns;
they are intentionally outside Paper Agent.

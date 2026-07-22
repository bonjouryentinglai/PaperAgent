# Phase 2A Move acceptance checklist

Use this checklist after installing the `phase-2a` branch on a Paper Pro Move.
Record the exact handwritten prompt and take a photo whenever a result fails.
Do not mark a device-sensitive feature complete from an ARM64 build alone.

## Test record

- Phase 2A implementation commit: `69d3999`
- Installed commit:
- Move OS version:
- Xochitl version:
- Test date:
- Result: [ ] Pass [ ] Fail

## Stop immediately if

- [ ] Xochitl crashes or the notebook cannot be reopened.
- [ ] The original selected content disappears.
- [ ] Paper Agent leaves the selection tool active or the pen unusable.
- [ ] One failed request prevents every later request from starting.

## Required smoke tests

### 1. Basic AI result

Write `2 + 3 等於多少？`, lasso it, and choose **AI**.

- [ ] Thinking appears, then disappears when the turn finishes.
- [ ] The answer is written below the selection as native ink.
- [ ] The answer is `5` and is readable at a normal size.
- [ ] Undo removes the result and Redo restores it.

### 2. Generic regular grids

Run all three prompts. These deliberately exercise one generic `grid` object,
not separate application code for each format.

1. `Draw a playable 9x9 Sudoku puzzle.`
2. `Draw a 7x7 crossword grid with several blocked cells.`
3. `Design a clean weekly calendar with seven day columns.`

- [ ] Every grid has straight, evenly spaced rows and columns.
- [ ] Sudoku cells are square, digits are centered, and each 3x3 boundary is
  visibly stronger than its inner lines.
- [ ] The crossword uses the requested grid size and visibly marks blocked cells.
- [ ] The weekly calendar uses seven columns with readable labels and room to write.
- [ ] No result falls back to prose or a generated raster picture.

The Sudoku test verifies drawing and layout only. Phase 2A does not yet prove
that a generated puzzle has exactly one mathematical solution.

### 3. Proportions in different spaces

Write and select the same request once with a narrow lasso and once with a wide
lasso: `Draw one circle, one square, and one wide ellipse.`

- [ ] The circle remains round in both results.
- [ ] The square keeps equal physical width and height.
- [ ] The ellipse is intentionally wider than it is tall.
- [ ] Shapes do not stretch merely to fill the available box.

### 4. Pen colors, widths, and restoration

First choose the primary pen, set a distinctive color and thickness, and draw a
small reference stroke. Then ask AI:

`Draw three horizontal lines: thin blue, medium red, and thick black.`

- [ ] All three lines use visibly different requested widths.
- [ ] Blue, red, and black are distinguishable.
- [ ] After Paper Agent finishes, a manual stroke uses the original color and
  thickness from before the request.
- [ ] Repeat the restoration check after cancellation and after a forced failure.

### 5. Text quality and alignment

Ask AI:

`Draw three equal cards. Write 左對齊 in the first, Center in the second, and 右對齊 in the third, using the matching alignment.`

- [ ] Left, center, and right alignment are visibly different and correct.
- [ ] Chinese and Latin text are both legible.
- [ ] Repeated copies of the same character have consistent geometry.
- [ ] Text stays inside its assigned card and does not add unintended line breaks.

### 6. Beautify text fidelity

Handwrite exactly three short lines, for example:

```text
第一行
Second line
第三行
```

Lasso all three and choose **Beautify**.

- [ ] The source remains unchanged.
- [ ] The result contains exactly the same words.
- [ ] Three source lines produce exactly three result lines.
- [ ] Beautify does not answer, translate, summarize, or correct the text.

### 7. Beautify geometry

Draw a rough mind map with one central box, two circles, two arrows, and short
labels. Make the circles intentionally uneven and the box slightly crooked.

- [ ] The source remains unchanged.
- [ ] Every node, label, connection, and arrow direction is preserved.
- [ ] Circles become round, the box becomes level, and connectors become straight.
- [ ] Repeated nodes have consistent size and spacing.

### 8. Image routing regression

Ask AI: `Generate a watercolor picture of a red bicycle beside a lake.`

- [ ] Thinking changes to image generation/insertion status.
- [ ] The result is a native image object, not native line art or prose.
- [ ] The image is placed below the selection and does not cover it.
- [ ] The image can be moved, resized, undone, and redone.

### 9. Cancellation and recovery

Start an image request and press **Cancel** while it is still generating.

- [ ] Cancellation is acknowledged and no partial ink or image is inserted.
- [ ] The original selection remains unchanged.
- [ ] The original pen color and thickness are restored.
- [ ] A new short AI request works immediately afterward.

### 10. Failure and recovery

Temporarily disconnect Wi-Fi, start one AI request, and wait for failure. Restore
Wi-Fi, then ask `1 + 1 = ?`.

- [ ] Failure is shown once without an automatic retry loop.
- [ ] Xochitl remains usable and the pen settings are restored.
- [ ] The request after Wi-Fi recovery succeeds.

## Persistence checks

Use at least one Scene result and one generated image for these checks.

- [ ] Lasso-select, move, and resize the result.
- [ ] Undo and Redo behave normally.
- [ ] Close and reopen the notebook; the result remains.
- [ ] Restart Xochitl or reboot the Move; the result remains.
- [ ] Export the page; the result appears in the export.
- [ ] If sync is enabled, the result appears correctly on another client.

## What to report for a failure

- Exact handwritten prompt and whether **AI** or **Beautify** was used.
- A photo showing the selection and result.
- Which numbered checkbox failed.
- Whether Cancel, Undo, pen input, and the next Paper Agent request still work.
- Approximate time from tapping the action until the first visible result.

---
name: structured-drawing
description: Design readable native-ink scenes without game-specific templates.
---

# Structured drawing

- Describe meaning with Scene objects; do not imitate rough pixels.
- Use one `grid` for any regular matrix such as a table, Sudoku, calendar, or
  planner. Set `majorEvery` when some divisions need stronger hierarchy.
- Use `rect`, `circle`, `ellipse`, `line`, `arrow`, and `polyline` for diagrams.
- Keep circles circular, squares square, repeated nodes equal-sized, lines level,
  labels centered in generous boxes, and margins consistent.
- Use thin lines for subdivisions, medium lines for ordinary outlines, and thick
  lines only for boundaries or emphasis.
- Use supported colors sparingly. Black must remain readable without color.
- Make the Scene canvas aspect ratio match the intended composition. The local
  renderer preserves proportions when fitting it to the available notebook box.
- This is generic layout guidance. Do not enumerate special cases for individual
  games, diagrams, or documents.

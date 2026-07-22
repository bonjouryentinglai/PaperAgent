// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import test from "node:test";
import { compileScene, validateSceneToolCall } from "./scene.mjs";

test("a semantic Sudoku becomes native grid runs with major lines", () => {
  validateSceneToolCall({
    version: 1,
    canvas: { width: 900, height: 900 },
    objects: [{
      type: "grid",
      group: "puzzle",
      x: 0, y: 0, width: 900, height: 900,
      rows: 9, columns: 9, majorEvery: 3,
      strokeWidth: "thin", majorStrokeWidth: "thick", color: "black",
      cells: [
        { row: 0, column: 0, text: "5" },
        { row: 0, column: 1, text: "3" },
        { row: 1, column: 3, text: "1", color: "blue" },
      ],
    }],
  });
  const runs = compileScene({
    version: 1,
    canvas: { width: 900, height: 900 },
    objects: [{
      type: "grid",
      group: "puzzle",
      x: 0, y: 0, width: 900, height: 900,
      rows: 9, columns: 9, majorEvery: 3,
      strokeWidth: "thin", majorStrokeWidth: "thick", color: "black",
      cells: [
        { row: 0, column: 0, text: "5" },
        { row: 0, column: 1, text: "3" },
        { row: 1, column: 3, text: "1", color: "blue" },
      ],
    }],
  }, { width: 760, height: 1_100 });
  assert.ok(runs.some((run) => run.style.width === "thin"));
  assert.ok(runs.some((run) => run.style.width === "thick"));
  assert.ok(runs.some((run) => run.style.color === "blue"));
  assert.ok(runs.every((run) => run.groups.includes("puzzle")));
  assert.ok(runs.length <= 4, "style batching should not create one writer job per grid line");
  assert.ok(runs.every((run) => run.body.startsWith("paper-agent-vector 1\n")));
});

test("contain mapping preserves a circle inside a portrait destination", () => {
  validateSceneToolCall({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [{ type: "circle", cx: 500, cy: 500, radius: 300 }],
  });
  const [run] = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [{ type: "circle", cx: 500, cy: 500, radius: 300 }],
  }, { width: 600, height: 1_200 });
  const match = run.body.match(/ellipse\s+\d+\s+\d+\s+(\d+)\s+(\d+)/u);
  assert.ok(match);
  const physicalRx = Number(match[1]) * (600 - 21) / 1_000;
  const physicalRy = Number(match[2]) * (1_200 - 21) / 1_000;
  assert.ok(Math.abs(physicalRx - physicalRy) <= 2);
});

test("unknown fields, duplicate cells, and out-of-bounds shapes fail closed", () => {
  const base = { version: 1, canvas: { width: 500, height: 500 } };
  assert.throws(() => validateSceneToolCall({
    ...base,
    objects: [{ type: "line", x1: 0, y1: 0, x2: 500, y2: 500, shell: "no" }],
  }), /unsupported field/u);
  assert.throws(() => validateSceneToolCall({
    ...base,
    objects: [{
      type: "grid", x: 0, y: 0, width: 500, height: 500, rows: 2, columns: 2,
      cells: [{ row: 0, column: 0, text: "A" }, { row: 0, column: 0, text: "B" }],
    }],
  }), /duplicate cell/u);
  assert.throws(() => validateSceneToolCall({
    ...base,
    objects: [{ type: "circle", cx: 20, cy: 20, radius: 40 }],
  }), /outside/u);
});

test("multiline text remains explicit lines and model text is never executable", () => {
  validateSceneToolCall({
    version: 1,
    canvas: { width: 1_000, height: 500 },
    objects: [{
      type: "text", x: 0, y: 0, width: 1_000, height: 500,
      text: "第一行\n$(touch /tmp/no) && second",
    }],
  });
  const [run] = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 500 },
    objects: [{
      type: "text", x: 0, y: 0, width: 1_000, height: 500,
      text: "第一行\n$(touch /tmp/no) && second",
    }],
  }, { width: 900, height: 500 });
  assert.match(run.body, /label .* 第一行/u);
  assert.match(run.body, /label .* \$\(touch \/tmp\/no\) && second/u);
});

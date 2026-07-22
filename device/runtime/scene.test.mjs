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

test("a safe shallow text Scene is accepted", () => {
  const scene = {
    version: 1,
    canvas: { width: 585, height: 81 },
    objects: [{
      type: "text", x: 18, y: 14, width: 550, height: 54,
      text: "明月明月，月光月色", align: "left",
    }],
  };
  validateSceneToolCall(scene, "beautify");
  const [run] = compileScene(scene, { width: 800, height: 300 }, "beautify");
  assert.deepEqual(run, {
    kind: "beautifyText",
    style: { color: "black", width: "medium" },
    body: "明月明月，月光月色",
    groups: [],
  });
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
  assert.equal(run.kind, "bodyText");
  assert.equal(run.body, "第一行\n$(touch /tmp/no) && second");
});

test("a lone AI answer becomes Scene body text instead of a shrinking label", () => {
  const [run] = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [{
      type: "text", x: 180, y: 420, width: 360, height: 40,
      text: "I'm Paper Agent,\nthe notebook assistant.", align: "left",
    }],
  }, { width: 900, height: 1_300 });
  assert.deepEqual(run, {
    kind: "bodyText",
    style: { color: "black", width: "medium" },
    body: "I'm Paper Agent,\nthe notebook assistant.",
    groups: [],
  });
});

test("a multi-object prose list becomes paginatable Scene flow text", () => {
  const [run] = compileScene({
    version: 1,
    layout: "flow",
    canvas: { width: 1_000, height: 600 },
    objects: [
      { type: "text", x: 100, y: 20, width: 800, height: 80, text: "改善睡眠的方法" },
      { type: "line", x1: 100, y1: 110, x2: 900, y2: 110 },
      { type: "text", x: 100, y: 140, width: 360, height: 80, text: "1. 固定作息" },
      { type: "text", x: 540, y: 140, width: 360, height: 80, text: "2. 睡前放鬆" },
    ],
  }, { width: 800, height: 300 });
  assert.deepEqual(run, {
    kind: "bodyText",
    style: { color: "black", width: "medium" },
    body: "改善睡眠的方法\n1. 固定作息\n2. 睡前放鬆",
    groups: [],
  });
});

test("plain text and dividers are forced into flow even when the model requests spatial", () => {
  const [run] = compileScene({
    version: 1,
    layout: "spatial",
    canvas: { width: 1_000, height: 600 },
    objects: [
      { type: "text", x: 100, y: 20, width: 800, height: 80, text: "改善睡眠的方法" },
      { type: "line", x1: 100, y1: 110, x2: 900, y2: 110 },
      { type: "text", x: 100, y: 140, width: 360, height: 80, text: "1. 固定作息" },
      { type: "text", x: 540, y: 140, width: 360, height: 80, text: "2. 睡前放鬆" },
    ],
  }, { width: 800, height: 300 });
  assert.deepEqual(run, {
    kind: "bodyText",
    style: { color: "black", width: "medium" },
    body: "改善睡眠的方法\n1. 固定作息\n2. 睡前放鬆",
    groups: [],
  });
});

test("a real spatial primitive keeps positioned text and geometry", () => {
  const runs = compileScene({
    version: 1,
    layout: "flow",
    canvas: { width: 1_000, height: 600 },
    objects: [
      { type: "rect", x: 100, y: 20, width: 800, height: 200 },
      { type: "text", x: 120, y: 80, width: 760, height: 80, text: "Card" },
    ],
  }, { width: 800, height: 300 });
  assert.ok(runs.every((run) => run.kind === "vector"));
  assert.ok(runs.some((run) => /rect /u.test(run.body)));
});

test("flow layout without text fails closed", () => {
  assert.throws(() => validateSceneToolCall({
    version: 1,
    layout: "flow",
    canvas: { width: 500, height: 500 },
    objects: [{ type: "line", x1: 0, y1: 100, x2: 500, y2: 100 }],
  }), /requires text/u);
});

test("Beautify text uses lasso-fit typography while diagram labels keep Scene placement", () => {
  const text = {
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [{ type: "text", x: 100, y: 400, width: 400, height: 100, text: "原文" }],
  };
  const [beautify] = compileScene(text, { width: 800, height: 1_200 }, "beautify");
  assert.equal(beautify.kind, "beautifyText");
  assert.equal(beautify.body, "原文");

  const [diagram] = compileScene({
    ...text,
    objects: [
      { type: "rect", x: 100, y: 400, width: 400, height: 100 },
      ...text.objects,
    ],
  }, { width: 800, height: 1_200 });
  assert.equal(diagram.kind, "vector");
  const diagramLabel = diagram.body.split("\n").find((line) => line.startsWith("label "));
  assert.match(diagramLabel, /^label 100 4\d\d 400 \d+ 原文$/u);
});

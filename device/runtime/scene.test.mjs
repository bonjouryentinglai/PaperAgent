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

test("harmless provider canvas metadata is discarded before rendering", () => {
  const scene = validateSceneToolCall({
    version: 1,
    canvas: {
      width: 600,
      height: 800,
      description: "portrait notebook canvas",
      origin: { x: 0, y: 0 },
    },
    objects: [{ type: "line", x1: 0, y1: 0, x2: 600, y2: 800 }],
  });
  assert.deepEqual(scene.canvas, { width: 600, height: 800 });
  assert.equal(Object.hasOwn(scene.canvas, "description"), false);
  assert.equal(Object.hasOwn(scene.canvas, "origin"), false);
  assert.throws(
    () => validateSceneToolCall({
      version: 1,
      canvas: { width: 600, height: 800, metadata: { nested: { value: "unsafe" } } },
      objects: [{ type: "line", x1: 0, y1: 0, x2: 600, y2: 800 }],
    }),
    /canvas metadata/u,
  );
});

test("semantic arcs and Bezier curves compile without model-authored polyline points", () => {
  const runs = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [
      {
        type: "arc",
        cx: 500, cy: 500, rx: 300, ry: 300,
        startAngle: 180, sweepAngle: 180,
        strokeWidth: "thick",
      },
      {
        type: "quadratic",
        start: { x: 100, y: 700 },
        control: { x: 500, y: 200 },
        end: { x: 900, y: 700 },
        color: "blue",
      },
      {
        type: "cubic",
        start: { x: 100, y: 800 },
        control1: { x: 300, y: 300 },
        control2: { x: 700, y: 900 },
        end: { x: 900, y: 400 },
      },
    ],
  }, { width: 600, height: 1_200 });
  assert.ok(runs.some((run) => /ellarc 500 500 300 147 180 180/u.test(run.body)));
  assert.ok(runs.some((run) => /curve \d+ \d+ \d+ \d+ \d+ \d+$/mu.test(run.body)));
  assert.ok(runs.some((run) => /curve \d+ \d+ \d+ \d+ \d+ \d+ \d+ \d+$/mu.test(run.body)));
  assert.ok(runs.some((run) => run.style.width === "thick"));
  assert.ok(runs.some((run) => run.style.color === "blue"));
  assert.ok(runs.every((run) => !/polyline/u.test(run.body)));
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
  assert.throws(() => validateSceneToolCall({
    ...base,
    objects: [{
      type: "arc", cx: 250, cy: 250, rx: 200, ry: 200,
      startAngle: 180, sweepAngle: 0,
    }],
  }), /non-zero/u);
  assert.throws(() => validateSceneToolCall({
    ...base,
    objects: [{
      type: "quadratic",
      start: { x: 0, y: 0 },
      control: { x: 250, y: 501 },
      end: { x: 500, y: 0 },
    }],
  }), /within 0..=500/u);
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

test("multi-object Beautify text is merged in visual line order", () => {
  const runs = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 600 },
    objects: [
      { type: "text", x: 100, y: 300, width: 800, height: 80, text: "第三行", group: "line-3" },
      { type: "text", x: 100, y: 60, width: 800, height: 80, text: "第一行", group: "line-1" },
      { type: "text", x: 100, y: 180, width: 800, height: 80, text: "第二行", group: "line-2" },
    ],
  }, { width: 800, height: 500 }, "beautify");
  assert.deepEqual(runs, [{
    kind: "beautifyText",
    style: { color: "black", width: "medium" },
    body: "第一行\n第二行\n第三行",
    groups: ["line-1", "line-2", "line-3"],
  }]);
});

test("mixed Beautify content keeps one spatial Scene path", () => {
  const runs = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 600 },
    objects: [
      { type: "rect", x: 100, y: 100, width: 800, height: 400 },
      { type: "text", x: 180, y: 240, width: 640, height: 100, text: "保留位置" },
    ],
  }, { width: 800, height: 500 }, "beautify");
  assert.ok(runs.every((run) => run.kind === "vector"));
  assert.ok(runs.some((run) => /rect /u.test(run.body)));
  assert.ok(runs.some((run) => /label /u.test(run.body)));
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

test("large legitimate Scenes are split into bounded native command batches", () => {
  const lines = Array.from({ length: 192 }, (_, index) => ({
    type: "line",
    x1: 0,
    y1: index,
    x2: 1_000,
    y2: index,
  }));
  const runs = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: lines,
  }, { width: 800, height: 1_200 });
  assert.equal(runs.length, 2);
  assert.ok(runs.every((run) => run.body.trim().split("\n").length - 1 <= 96));
});

test("expanded Scene totals are bounded before native writeback", () => {
  const populatedCells = Array.from({ length: 256 }, (_, index) => ({
    row: Math.floor(index / 16),
    column: index % 16,
    text: "1",
  }));
  const denseGrid = {
    type: "grid",
    x: 0,
    y: 0,
    width: 1_000,
    height: 1_000,
    rows: 16,
    columns: 16,
    cells: populatedCells,
  };
  const denseScene = {
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [denseGrid, denseGrid],
  };
  validateSceneToolCall(denseScene);
  assert.throws(
    () => compileScene(denseScene, { width: 800, height: 1_200 }),
    /512 native commands/u,
  );

  const path = Array.from({ length: 64 }, (_, index) => ({
    x: index,
    y: index,
  }));
  assert.throws(() => validateSceneToolCall({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: Array.from({ length: 65 }, () => ({
      type: "polyline",
      points: path,
    })),
  }), /4096 path control points/u);
});

test("long spatial labels remain bounded Scene labels for native wrapping", () => {
  const text = "This is a deliberately long diagram annotation that should wrap inside its label box instead of failing the old two-hundred-and-forty character Scene limit. ".repeat(3).trim();
  assert.ok([...text].length > 240);
  const runs = compileScene({
    version: 1,
    canvas: { width: 1_000, height: 1_000 },
    objects: [
      { type: "rect", x: 50, y: 50, width: 900, height: 900 },
      { type: "text", x: 100, y: 100, width: 800, height: 800, text, align: "left" },
    ],
  }, { width: 800, height: 1_200 });
  assert.ok(runs.some((run) => run.body.includes(text)));
});

test("text-heavy mixed Scenes become paginatable flow instead of oversized labels", () => {
  const methods = Array.from(
    { length: 30 },
    (_, index) => `${index + 1}. 方法 ${index + 1}。這是一段完整說明，並包含第二句補充。`,
  );
  const [result] = compileScene({
    version: 1,
    layout: "spatial",
    canvas: { width: 1_000, height: 1_600 },
    objects: [
      { type: "rect", x: 40, y: 40, width: 920, height: 1_520 },
      ...methods.map((text, index) => ({
        type: "text",
        x: index % 2 === 0 ? 80 : 520,
        y: 100 + Math.floor(index / 2) * 90,
        width: 400,
        height: 70,
        text,
        align: "left",
      })),
    ],
  }, { width: 800, height: 1_200 });
  assert.equal(result.kind, "bodyText");
  assert.equal(result.body.split("\n").length, 30);
  assert.match(result.body, /30\. 方法 30/u);
});

test("numbered two-column prose is restored to logical numeric order", () => {
  const objects = [{ type: "text", x: 100, y: 20, width: 800, height: 50, text: "改善睡眠" }];
  for (let index = 1; index <= 20; index += 1) {
    const rightColumn = index > 10;
    objects.push({
      type: "text",
      x: rightColumn ? 520 : 80,
      y: 100 + ((index - 1) % 10) * 100,
      width: 400,
      height: 80,
      text: `${index}. 方法 ${index}。這是一段足以觸發流式排版的完整說明。`,
      align: "left",
    });
  }
  const [result] = compileScene({
    version: 1,
    layout: "spatial",
    canvas: { width: 1_000, height: 1_200 },
    objects,
  }, { width: 800, height: 1_200 });
  assert.equal(result.kind, "bodyText");
  const numbers = result.body
    .split("\n")
    .map((line) => {
      const match = line.match(/^(\d{1,3})[.)、．]/u);
      return match ? Number(match[1]) : null;
    })
    .filter((value) => value !== null);
  assert.deepEqual(numbers, Array.from({ length: 20 }, (_, index) => index + 1));
});

test("long diagram labels use the enclosing rectangle interior", () => {
  const runs = compileScene({
    version: 1,
    layout: "spatial",
    canvas: { width: 1_000, height: 1_000 },
    objects: [
      { type: "rect", x: 100, y: 200, width: 800, height: 300, radius: 20 },
      {
        type: "text",
        x: 160,
        y: 310,
        width: 680,
        height: 60,
        text: "First, identify the complete problem that you want to solve before choosing an action.",
      },
    ],
  }, { width: 800, height: 1_200 });
  const label = runs
    .flatMap((run) => run.body.split("\n"))
    .find((line) => line.startsWith("label "));
  assert.ok(label);
  const [, x, y, width, height] = label.split(/\s+/u).map(Number);
  assert.ok(x > 100 && y > 200);
  assert.ok(width > 700);
  assert.ok(height > 150, "the label should use the enclosing box height for wrapping");
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

// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import test from "node:test";
import { compileRichDocument, parseRichDocument, validateVectorBody } from "./rich-document.mjs";

test("keeps headings, paragraphs, lists, inline styles, and fenced code in rich blocks", () => {
  const markdown = [
    "# 結果",
    "",
    "一般段落包含 **粗體** 與 `inline code`。",
    "",
    "- 第一項",
    "- 第二項",
    "",
    "```js",
    "const answer = 42;",
    "```",
  ].join("\n");
  const blocks = parseRichDocument(markdown);
  assert.deepEqual(blocks.map((block) => block.kind), ["rich"]);
  assert.match(blocks[0].body, /\*\*粗體\*\*/u);
  assert.match(blocks[0].body, /```js/u);
});

test("preserves mixed block order for Markdown tables and safe vectors", () => {
  const markdown = [
    "## 摘要",
    "這是說明。",
    "",
    "| 項目 | 數值 |",
    "| --- | ---: |",
    "| A | 10 |",
    "",
    "接著是圖：",
    "",
    "```paper-agent-vector",
    "paper-agent-vector 1",
    "rect 100 100 300 200",
    "arrow 400 200 800 700",
    "label 100 50 300 100 Start here",
    "```",
    "",
    "完成。",
  ].join("\n");
  const blocks = parseRichDocument(markdown);
  assert.deepEqual(blocks.map((block) => block.kind), ["rich", "table", "rich", "vector", "rich"]);
  const manifest = compileRichDocument(markdown);
  assert.match(manifest, /^paper-agent-document 1\n/u);
  assert.equal(manifest.split("\n").filter((line) => line.startsWith("block ")).length, 5);
  assert.doesNotMatch(manifest, /Start here/u, "manifest payload must remain encoded data");
});

test("recognizes a bounded pipe table without a Markdown separator", () => {
  const blocks = parseRichDocument([
    "摘要：",
    "",
    "| 項目 | 數值 |",
    "| A | 10 |",
    "| B | 20 |",
    "",
    "完成。",
  ].join("\n"));
  assert.deepEqual(blocks.map((block) => block.kind), ["rich", "table", "rich"]);
  assert.match(blocks[1].body, /\| B \| 20 \|/u);
});

test("ordinary fenced code is data and is never reclassified as a vector", () => {
  const blocks = parseRichDocument("```svg\n<script>alert(1)</script>\n```");
  assert.deepEqual(blocks.map((block) => block.kind), ["rich"]);
});

test("rejects unsafe or malformed vector fences", () => {
  assert.throws(() => validateVectorBody("paper-agent-vector 1\nsvg <script/>"), /unsupported vector command/u);
  assert.throws(() => parseRichDocument("```vector\npaper-agent-vector 1\nline 0 0 1001 2\n```"), /outside/u);
  assert.throws(() => parseRichDocument("```vector\npaper-agent-vector 1\nline 0 0 1 2"), /unterminated/u);
});

test("accepts the expanded bounded curve, shape, and hatch primitives", () => {
  const vector = [
    "paper-agent-vector 1",
    "polygon 100 100 300 100 200 300",
    "curve 100 200 200 100 300 200",
    "curve 100 300 200 100 300 500 400 300",
    "rrect 100 100 300 200 40",
    "arc 500 500 120 0 180",
    "ellarc 500 500 200 120 180 180",
    "wedge 500 500 120 180 360",
    "dot 500 500",
    "fillpoly 100 500 250 400 400 500",
    "box 600 100 200 150",
    "disc 700 500 80",
    "fillellipse 300 700 160 80",
    "labelleft 20 850 220 80 Left",
    "labelright 760 850 220 80 Right",
  ].join("\n");
  assert.equal(validateVectorBody(vector), vector);
});

test("bounds expanded vectors before native rendering", () => {
  const tooManyPoints = Array.from({ length: 65 }, (_, index) => `${index} ${index}`).join(" ");
  assert.throws(
    () => validateVectorBody(`paper-agent-vector 1\npolygon ${tooManyPoints}`),
    /64 points/u,
  );
  assert.throws(
    () => validateVectorBody("paper-agent-vector 1\narc 500 500 100 0 361"),
    /angle outside/u,
  );
  assert.throws(
    () => validateVectorBody("paper-agent-vector 1\nellarc 500 500 100 80 180 0"),
    /nonzero|non-zero/u,
  );
  assert.throws(
    () => validateVectorBody("paper-agent-vector 1\nrrect 900 900 200 100 20"),
    /extends outside/u,
  );
  assert.throws(
    () => validateVectorBody(["paper-agent-vector 1", ...Array(16).fill("box 0 0 1000 1000")].join("\n")),
    /512 strokes/u,
  );
});

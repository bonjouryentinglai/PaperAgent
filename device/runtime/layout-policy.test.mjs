// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import test from "node:test";
import {
  chooseLargestFittingScale,
  safePagePlacement,
} from "./layout-policy.mjs";

test("keeps the default scale when it fits", async () => {
  const seen = [];
  const scale = await chooseLargestFittingScale(async (candidate) => {
    seen.push(candidate);
    return true;
  });
  assert.equal(scale, 100);
  assert.deepEqual(seen, [100]);
});

test("chooses the largest fitting scale without going below sixty percent", async () => {
  const scale = await chooseLargestFittingScale(async (candidate) => candidate <= 83);
  assert.equal(scale, 83);
  const page = await chooseLargestFittingScale(async (candidate) => candidate < 60);
  assert.equal(page, null);
});

test("supports a configurable default size and relative minimum", async () => {
  const scale = await chooseLargestFittingScale(
    async (candidate) => candidate <= 91,
    { defaultScale: 120, minimumScale: 72 },
  );
  assert.equal(scale, 91);
  const page = await chooseLargestFittingScale(
    async (candidate) => candidate < 72,
    { defaultScale: 120, minimumScale: 72 },
  );
  assert.equal(page, null);
});

test("rejects invalid scale ranges", async () => {
  await assert.rejects(
    chooseLargestFittingScale(async () => true, { defaultScale: 80, minimumScale: 90 }),
    /minimumScale/u,
  );
});

test("new AI pages use the full safe area while Beautify preserves its lasso size", () => {
  assert.deepEqual(safePagePlacement("ai", { x: 1, y: 2, width: 3, height: 4 }), {
    x: 50, y: 150, width: 854, height: 1496,
  });
  assert.deepEqual(safePagePlacement("beautify", {
    x: 700, y: 1200, width: 300, height: 220,
  }), {
    x: 604, y: 150, width: 300, height: 220,
  });
});

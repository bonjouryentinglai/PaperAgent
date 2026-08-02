// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import {
  applySettings,
  DEFAULT_SETTINGS,
  parseSettings,
  restartPaperAgent,
  updateSettings,
  validateSettings,
} from "./settings-controller.mjs";

test("reads defaults and preserves unrelated settings", () => {
  assert.deepEqual(parseSettings("PAPER_AGENT_IMAGE_QUALITY=low\n"), DEFAULT_SETTINGS);
  const updated = updateSettings(
    "PAPER_AGENT_IMAGE_QUALITY=low\nPAPER_AGENT_MODEL=old\n",
    { ...DEFAULT_SETTINGS, model: "gpt-5.6-luna", textScalePercent: 110 },
  );
  assert.match(updated, /^PAPER_AGENT_IMAGE_QUALITY=low$/mu);
  assert.match(updated, /^PAPER_AGENT_MODEL=gpt-5\.6-luna$/mu);
  assert.match(updated, /^PAPER_AGENT_TEXT_SCALE_PERCENT=110$/mu);
  assert.equal((updated.match(/PAPER_AGENT_MODEL=/gu) || []).length, 1);
});

test("rejects unsafe and out-of-range values", () => {
  assert.throws(
    () => validateSettings({ ...DEFAULT_SETTINGS, model: "gpt; reboot" }),
    /model/u,
  );
  assert.throws(
    () => validateSettings({ ...DEFAULT_SETTINGS, textScalePercent: 200 }),
    /textScalePercent/u,
  );
  assert.throws(
    () => validateSettings({ ...DEFAULT_SETTINGS, imageQuality: "high" }),
    /unsupported fields/u,
  );
});

test("applies atomically and rolls back an unhealthy service", async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "paper-agent-settings-"));
  const configPath = path.join(directory, "config.env");
  const lockPath = path.join(directory, "settings.lock");
  const previous = updateSettings("PAPER_AGENT_IMAGE_QUALITY=low\n", DEFAULT_SETTINGS);
  fs.writeFileSync(configPath, previous, { mode: 0o600 });
  let restarts = 0;
  await assert.rejects(
    applySettings(
      { ...DEFAULT_SETTINGS, model: "gpt-5.6-luna" },
      {
        configPath,
        lockPath,
        queryState: async () => ({ active: false }),
        restartService: async () => { restarts += 1; },
        waitForHealth: async (settings) => {
          if (settings.model === "gpt-5.6-luna") throw new Error("not ready");
          return { ready: true };
        },
      },
    ),
    /restored/u,
  );
  assert.equal(restarts, 2);
  assert.equal(fs.readFileSync(configPath, "utf8"), previous);
  assert.equal(fs.existsSync(lockPath), false);
});

test("does not modify settings while an AI request is active", async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "paper-agent-settings-busy-"));
  const configPath = path.join(directory, "config.env");
  const lockPath = path.join(directory, "settings.lock");
  fs.writeFileSync(configPath, updateSettings("", DEFAULT_SETTINGS));
  await assert.rejects(
    applySettings(DEFAULT_SETTINGS, {
      configPath,
      lockPath,
      queryState: async () => ({ active: true }),
      restartService: async () => assert.fail("must not restart"),
    }),
    /current Paper Agent request/u,
  );
});

test("manually restarts the service without changing settings", async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "paper-agent-settings-restart-"));
  const configPath = path.join(directory, "config.env");
  const lockPath = path.join(directory, "settings.lock");
  const contents = updateSettings("PAPER_AGENT_IMAGE_QUALITY=low\n", {
    ...DEFAULT_SETTINGS,
    model: "gpt-5.6-luna",
  });
  fs.writeFileSync(configPath, contents, { mode: 0o600 });
  let restarts = 0;
  let expected;
  const ready = await restartPaperAgent({
    configPath,
    lockPath,
    restartService: async () => { restarts += 1; },
    waitForHealth: async (settings) => {
      expected = settings;
      return { type: "ready", ready: true, active: false };
    },
  });
  assert.equal(restarts, 1);
  assert.equal(expected.model, "gpt-5.6-luna");
  assert.equal(ready.ready, true);
  assert.equal(fs.readFileSync(configPath, "utf8"), contents);
  assert.equal(fs.existsSync(lockPath), false);
});

test("releases the settings lock when a manual restart fails", async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "paper-agent-settings-restart-fail-"));
  const configPath = path.join(directory, "config.env");
  const lockPath = path.join(directory, "settings.lock");
  fs.writeFileSync(configPath, updateSettings("", DEFAULT_SETTINGS));
  await assert.rejects(
    restartPaperAgent({
      configPath,
      lockPath,
      restartService: async () => { throw new Error("restart failed"); },
    }),
    /restart failed/u,
  );
  assert.equal(fs.existsSync(lockPath), false);
});

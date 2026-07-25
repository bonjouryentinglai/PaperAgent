#!/usr/bin/env node
// SPDX-License-Identifier: MIT
//
// Narrow settings transaction used by the AppLoad Settings backend. It owns
// only four allowlisted values, never reads OAuth credentials, and keeps the
// service restart rollback-safe.

import { execFile } from "node:child_process";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";
import process from "node:process";

export const DEFAULT_SETTINGS = Object.freeze({
  model: "gpt-5.6-sol",
  thinking: "off",
  textScalePercent: 100,
  minAutoScalePercent: 60,
});

export const MODEL_OPTIONS = Object.freeze([
  "gpt-5.6-sol",
  "gpt-5.6-luna",
]);

export const THINKING_OPTIONS = Object.freeze([
  "off", "minimal", "low", "medium", "high", "xhigh", "max",
]);

const MANAGED = Object.freeze({
  PAPER_AGENT_MODEL: "model",
  PAPER_AGENT_THINKING: "thinking",
  PAPER_AGENT_TEXT_SCALE_PERCENT: "textScalePercent",
  PAPER_AGENT_MIN_AUTO_SCALE_PERCENT: "minAutoScalePercent",
});
const CONFIG = process.env.PAPER_AGENT_CONFIG || "/home/root/paper-agent/config.env";
const SOCKET = process.env.PAPER_AGENT_NATIVE_SOCKET || "/run/paper-agent-native-oracle.sock";
const LOCK = process.env.PAPER_AGENT_SETTINGS_APPLY_LOCK
  || "/run/paper-agent-settings-apply.lock";
const SERVICE = "paper-agent-native-oracle.service";

function integer(value, name, minimum, maximum) {
  const parsed = typeof value === "number" ? value : Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < minimum || parsed > maximum) {
    throw new Error(`${name} must be between ${minimum} and ${maximum}`);
  }
  return parsed;
}

export function validateSettings(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error("settings must be an object");
  }
  const keys = Object.keys(value).sort();
  const expected = Object.keys(DEFAULT_SETTINGS).sort();
  if (keys.length !== expected.length || keys.some((key, index) => key !== expected[index])) {
    throw new Error("settings contain unsupported fields");
  }
  const model = String(value.model);
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,95}$/u.test(model)) {
    throw new Error("model contains unsupported characters");
  }
  const thinking = String(value.thinking);
  if (!THINKING_OPTIONS.includes(thinking)) {
    throw new Error("thinking is not supported");
  }
  return {
    model,
    thinking,
    textScalePercent: integer(value.textScalePercent, "textScalePercent", 70, 160),
    minAutoScalePercent: integer(value.minAutoScalePercent, "minAutoScalePercent", 40, 100),
  };
}

export function parseSettings(contents) {
  const settings = { ...DEFAULT_SETTINGS };
  for (const line of String(contents).split(/\r?\n/u)) {
    const match = line.match(/^([A-Z0-9_]+)=(.*)$/u);
    if (!match || !Object.hasOwn(MANAGED, match[1])) continue;
    const field = MANAGED[match[1]];
    settings[field] = match[2];
  }
  return validateSettings(settings);
}

export function updateSettings(contents, value) {
  const settings = validateSettings(value);
  const kept = String(contents)
    .split(/\r?\n/u)
    .filter((line) => {
      const match = line.match(/^([A-Z0-9_]+)=/u);
      return !match || !Object.hasOwn(MANAGED, match[1]);
    });
  while (kept.length > 0 && kept.at(-1) === "") kept.pop();
  kept.push(
    "",
    "# Managed by Paper Agent Settings.",
    `PAPER_AGENT_MODEL=${settings.model}`,
    `PAPER_AGENT_THINKING=${settings.thinking}`,
    `PAPER_AGENT_TEXT_SCALE_PERCENT=${settings.textScalePercent}`,
    `PAPER_AGENT_MIN_AUTO_SCALE_PERCENT=${settings.minAutoScalePercent}`,
    "",
  );
  return kept.join("\n");
}

function atomicWrite(filename, contents) {
  fs.mkdirSync(path.dirname(filename), { recursive: true, mode: 0o700 });
  const temporary = `${filename}.settings-${process.pid}-${Date.now()}`;
  let fd;
  try {
    fd = fs.openSync(temporary, "wx", 0o600);
    fs.writeFileSync(fd, contents, "utf8");
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    fd = undefined;
    fs.renameSync(temporary, filename);
    fs.chmodSync(filename, 0o600);
  } finally {
    if (fd !== undefined) fs.closeSync(fd);
    fs.rmSync(temporary, { force: true });
  }
}

function socketRequest(request, timeoutMs = 2_000) {
  return new Promise((resolve, reject) => {
    const socket = net.createConnection(SOCKET);
    let buffer = "";
    const timer = setTimeout(() => {
      socket.destroy();
      reject(new Error("Paper Agent service did not respond"));
    }, timeoutMs);
    socket.setEncoding("utf8");
    socket.on("connect", () => socket.write(`${JSON.stringify(request)}\n`));
    socket.on("data", (chunk) => {
      buffer += chunk;
      const newline = buffer.indexOf("\n");
      if (newline < 0) return;
      clearTimeout(timer);
      socket.end();
      try {
        resolve(JSON.parse(buffer.slice(0, newline)));
      } catch {
        reject(new Error("Paper Agent service returned invalid state"));
      }
    });
    socket.on("error", (error) => {
      clearTimeout(timer);
      reject(error);
    });
  });
}

function execFilePromise(file, args) {
  return new Promise((resolve, reject) => {
    execFile(file, args, { timeout: 15_000 }, (error, stdout, stderr) => {
      if (error) {
        reject(new Error(String(stderr || stdout || error.message).trim()));
      } else {
        resolve(stdout);
      }
    });
  });
}

async function restartService() {
  await execFilePromise("/bin/systemctl", ["restart", SERVICE]);
}

async function waitForHealth(expected) {
  let lastError;
  for (let attempt = 0; attempt < 40; attempt += 1) {
    try {
      const state = await socketRequest({ version: 1, type: "health" }, 1_000);
      if (state.type === "ready"
          && state.model === expected.model
          && state.thinking === expected.thinking
          && state.textScalePercent === expected.textScalePercent
          && state.minAutoScalePercent === expected.minAutoScalePercent) {
        return state;
      }
      lastError = new Error(state.error || "Paper Agent loaded different settings");
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw lastError || new Error("Paper Agent did not become ready");
}

function acquireLock(filename) {
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  const fd = fs.openSync(filename, "wx", 0o600);
  fs.writeFileSync(fd, `${process.pid}\n`);
  return () => {
    fs.closeSync(fd);
    fs.rmSync(filename, { force: true });
  };
}

export async function applySettings(value, overrides = {}) {
  const settings = validateSettings(value);
  const configPath = overrides.configPath || CONFIG;
  const lockPath = overrides.lockPath || LOCK;
  const state = overrides.queryState || (() => socketRequest({ version: 1, type: "state" }));
  const restart = overrides.restartService || restartService;
  const health = overrides.waitForHealth || waitForHealth;
  let unlock;
  try {
    unlock = acquireLock(lockPath);
  } catch (error) {
    if (error?.code === "EEXIST") throw new Error("Paper Agent settings are already being applied");
    throw error;
  }
  const existed = fs.existsSync(configPath);
  const previous = existed ? fs.readFileSync(configPath, "utf8") : "";
  const next = updateSettings(previous, settings);
  try {
    const currentState = await state();
    if (currentState.active) throw new Error("Wait for the current Paper Agent request to finish");
    if (next === previous) {
      return { settings, changed: false, state: currentState };
    }
    atomicWrite(configPath, next);
    try {
      await restart();
      const ready = await health(settings);
      return { settings, changed: true, state: ready };
    } catch (activationError) {
      if (existed) atomicWrite(configPath, previous);
      else fs.rmSync(configPath, { force: true });
      try {
        await restart();
        await health(parseSettings(previous));
      } catch (rollbackError) {
        throw new Error(
          `Activation failed (${activationError.message}); rollback failed (${rollbackError.message})`,
        );
      }
      throw new Error(`Settings were restored after activation failed: ${activationError.message}`);
    }
  } finally {
    unlock();
  }
}

export async function readState() {
  const contents = fs.existsSync(CONFIG) ? fs.readFileSync(CONFIG, "utf8") : "";
  const settings = parseSettings(contents);
  let service = { ready: false, active: false };
  try {
    service = await socketRequest({ version: 1, type: "state" });
  } catch {}
  const modelOptions = MODEL_OPTIONS.includes(settings.model)
    ? [...MODEL_OPTIONS]
    : [settings.model, ...MODEL_OPTIONS];
  return { settings, modelOptions, thinkingOptions: THINKING_OPTIONS, service };
}

async function readStdin() {
  let body = "";
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    body += chunk;
    if (body.length > 16_384) throw new Error("settings request is too large");
  }
  return body;
}

async function main() {
  const command = process.argv[2];
  if (command === "get") {
    process.stdout.write(`${JSON.stringify(await readState())}\n`);
    return;
  }
  if (command === "apply") {
    const result = await applySettings(JSON.parse(await readStdin()));
    process.stdout.write(`${JSON.stringify({ ...await readState(), changed: result.changed })}\n`);
    return;
  }
  throw new Error("usage: settings-controller.mjs get | apply");
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(new URL(import.meta.url).pathname)) {
  main().catch((error) => {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  });
}

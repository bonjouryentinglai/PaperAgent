#!/usr/bin/env node
// SPDX-License-Identifier: MIT
//
// Narrow ChatGPT-subscription image helper for Paper Agent.
//
// Authentication and refresh follow Pi 0.80.7's openai-codex OAuth provider;
// the Codex Responses image_generation request and SSE extraction follow
// OpenClaw's MIT-licensed OpenAI image provider, pinned at
// 28a3540f3283b0700ffde4ffaa0a5f7303d73a09. This process has no general agent
// tools: it accepts one prompt on stdin and writes one validated PNG.

import { randomUUID } from "node:crypto";
import {
  lstat,
  mkdir,
  open,
  readFile,
  rename,
  rm,
  stat,
  utimes,
  writeFile,
} from "node:fs/promises";
import { dirname } from "node:path";

const AUTH_PATH = process.env.PAPER_AGENT_PI_AUTH_PATH || "/home/root/.pi/agent/auth.json";
const PROVIDER = process.env.PAPER_AGENT_PROVIDER || "openai-codex";
const TOKEN_URL = "https://auth.openai.com/oauth/token";
const CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann";
const RESPONSES_URL = "https://chatgpt.com/backend-api/codex/responses";
const RESPONSES_MODEL = process.env.PAPER_AGENT_IMAGE_RESPONSES_MODEL || "gpt-5.6-sol";
const IMAGE_MODEL = process.env.PAPER_AGENT_IMAGE_MODEL || "gpt-image-2";
const IMAGE_SIZE = process.env.PAPER_AGENT_IMAGE_SIZE || "1024x1024";
const IMAGE_QUALITY = process.env.PAPER_AGENT_IMAGE_QUALITY || "low";
const MAX_PROMPT_CHARS = 32_000;
const MAX_SSE_BYTES = 64 * 1024 * 1024;
const MAX_PNG_BYTES = 32 * 1024 * 1024;
const REFRESH_SKEW_MS = 60_000;
const LOCK_WAIT_MS = 10_000;
const LOCK_RETRY_MS = 100;
const LOCK_STALE_MS = 60_000;
const LOCK_HEARTBEAT_MS = 5_000;
const OUTPUT_ROOT = "/home/root/paper-agent/native/artifacts";

const IMAGE_MODELS = new Set(["gpt-image-2"]);
const IMAGE_SIZES = new Set(["1024x1024", "1536x1024", "1024x1536"]);
const IMAGE_QUALITIES = new Set(["low", "medium", "high"]);

function fail(message) {
  process.stderr.write(`paper-agent image: ${message}\n`);
  process.exitCode = 1;
}

function outputPathFromArgs(argv) {
  const i = argv.indexOf("--output");
  if (i < 0 || !argv[i + 1]) throw new Error("usage: image-generate.mjs --output PATH");
  const output = argv[i + 1];
  const escaped = OUTPUT_ROOT.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  if (!new RegExp(`^${escaped}/[0-9]{10,20}\\.png$`).test(output)) {
    throw new Error("image output path is outside Paper Agent's artifact store");
  }
  return output;
}

async function readPrompt() {
  const chunks = [];
  let length = 0;
  for await (const chunk of process.stdin) {
    length += chunk.length;
    if (length > MAX_PROMPT_CHARS * 4) throw new Error("image prompt is too long");
    chunks.push(chunk);
  }
  const prompt = Buffer.concat(chunks).toString("utf8").trim();
  if (!prompt) throw new Error("image prompt is empty");
  if (prompt.length > MAX_PROMPT_CHARS) throw new Error("image prompt is too long");
  return prompt;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// proper-lockfile (used by Pi's AuthStorage) locks auth.json by atomically
// creating auth.json.lock. Use the same lock path so token rotation cannot
// overwrite Pi or be overwritten by its resident RPC process.
async function acquireAuthLock() {
  const lockPath = `${AUTH_PATH}.lock`;
  const deadline = Date.now() + LOCK_WAIT_MS;
  while (true) {
    try {
      await mkdir(lockPath, { mode: 0o700 });
      const heartbeat = setInterval(() => {
        const now = new Date();
        void utimes(lockPath, now, now).catch(() => undefined);
      }, LOCK_HEARTBEAT_MS);
      heartbeat.unref();
      return async () => {
        clearInterval(heartbeat);
        await rm(lockPath, { recursive: true, force: true });
      };
    } catch (error) {
      if (error?.code !== "EEXIST") {
        throw new Error("could not lock the Pi OAuth credential store");
      }
      try {
        const lock = await lstat(lockPath);
        if (lock.isDirectory() && !lock.isSymbolicLink() && Date.now() - lock.mtimeMs > LOCK_STALE_MS) {
          await rm(lockPath, { recursive: true });
          continue;
        }
      } catch (inspectError) {
        if (inspectError?.code === "ENOENT") continue;
        throw new Error("could not inspect the Pi OAuth credential lock");
      }
      if (Date.now() >= deadline) throw new Error("could not lock the Pi OAuth credential store");
      await sleep(LOCK_RETRY_MS);
    }
  }
}

async function validateAuthFile() {
  let info;
  try {
    info = await lstat(AUTH_PATH);
  } catch (error) {
    if (error?.code === "ENOENT") {
      throw new Error(`no usable ${PROVIDER} OAuth login; run the Paper Agent login again`);
    }
    throw new Error("could not inspect the Pi OAuth credential store");
  }
  if (!info.isFile() || info.isSymbolicLink()) {
    throw new Error("Pi OAuth credential store must be a regular file");
  }
  if (typeof info.uid === "number" && info.uid !== 0) {
    throw new Error("Pi OAuth credential store must be owned by root");
  }
  if ((info.mode & 0o077) !== 0) {
    throw new Error("Pi OAuth credential store permissions must not be broader than 0600");
  }
}

function decodeJwtPayload(token) {
  const pieces = token.split(".");
  if (pieces.length !== 3) throw new Error("Pi OAuth access token is malformed");
  return JSON.parse(Buffer.from(pieces[1], "base64url").toString("utf8"));
}

function accountIdFor(credential) {
  if (typeof credential.accountId === "string" && credential.accountId) {
    return credential.accountId;
  }
  const payload = decodeJwtPayload(credential.access);
  const id = payload?.["https://api.openai.com/auth"]?.chatgpt_account_id;
  if (typeof id !== "string" || !id) throw new Error("Pi OAuth token has no ChatGPT account id");
  return id;
}

function validateCredential(value) {
  if (
    value?.type !== "oauth" ||
    typeof value.access !== "string" ||
    typeof value.refresh !== "string" ||
    typeof value.expires !== "number"
  ) {
    throw new Error(`no usable ${PROVIDER} OAuth login; run the Paper Agent login again`);
  }
  return value;
}

async function refreshCredential(current) {
  const response = await fetch(TOKEN_URL, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "refresh_token",
      refresh_token: current.refresh,
      client_id: CLIENT_ID,
    }),
    signal: AbortSignal.timeout(30_000),
  });
  if (!response.ok) {
    throw new Error(`ChatGPT login refresh failed (${response.status})`);
  }
  const token = await response.json();
  if (
    typeof token?.access_token !== "string" ||
    typeof token?.refresh_token !== "string" ||
    typeof token?.expires_in !== "number"
  ) {
    throw new Error("ChatGPT login refresh returned incomplete credentials");
  }
  const next = {
    type: "oauth",
    access: token.access_token,
    refresh: token.refresh_token,
    expires: Date.now() + token.expires_in * 1_000,
  };
  next.accountId = accountIdFor(next);
  return next;
}

async function usableCredential() {
  await mkdir(dirname(AUTH_PATH), { recursive: true, mode: 0o700 });
  await validateAuthFile();
  const release = await acquireAuthLock();
  try {
    await validateAuthFile();
    const store = JSON.parse(await readFile(AUTH_PATH, "utf8"));
    let credential = validateCredential(store[PROVIDER]);
    if (credential.expires <= Date.now() + REFRESH_SKEW_MS) {
      credential = await refreshCredential(credential);
      store[PROVIDER] = credential;
      const temporary = `${AUTH_PATH}.paper-agent-${randomUUID()}`;
      await writeFile(temporary, `${JSON.stringify(store, null, 2)}\n`, {
        mode: 0o600,
        flag: "wx",
      });
      await rename(temporary, AUTH_PATH);
      const handle = await open(AUTH_PATH, "r");
      try {
        await handle.chmod(0o600);
      } finally {
        await handle.close();
      }
    }
    return { access: credential.access, accountId: accountIdFor(credential) };
  } finally {
    await release();
  }
}

async function readLimitedText(response) {
  const declared = Number(response.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > MAX_SSE_BYTES) {
    throw new Error("image response exceeded the size limit");
  }
  if (!response.body) return "";
  const reader = response.body.getReader();
  const chunks = [];
  let length = 0;
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    length += value.byteLength;
    if (length > MAX_SSE_BYTES) {
      await reader.cancel().catch(() => undefined);
      throw new Error("image response exceeded the size limit");
    }
    chunks.push(value);
  }
  return Buffer.concat(chunks).toString("utf8");
}

function imagePayloadFromSse(body) {
  let fallback;
  for (const line of body.split(/\r?\n/)) {
    if (!line.startsWith("data:")) continue;
    const data = line.slice(5).trim();
    if (!data || data === "[DONE]") continue;
    let event;
    try {
      event = JSON.parse(data);
    } catch {
      continue;
    }
    if (event.type === "response.failed" || event.type === "error") {
      throw new Error("ChatGPT image generation failed");
    }
    if (
      event.type === "response.output_item.done" &&
      event.item?.type === "image_generation_call" &&
      typeof event.item.result === "string"
    ) {
      return event.item.result;
    }
    if (event.type === "response.completed") {
      const item = event.response?.output?.find(
        (entry) => entry?.type === "image_generation_call" && typeof entry.result === "string",
      );
      if (item) fallback = item.result;
    }
  }
  if (fallback) return fallback;
  throw new Error("ChatGPT returned no generated image");
}

async function generatePng(prompt, credential) {
  const response = await fetch(RESPONSES_URL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${credential.access}`,
      "chatgpt-account-id": credential.accountId,
      originator: "pi",
      "OpenAI-Beta": "responses=experimental",
      Accept: "text/event-stream",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(buildRequestBody(prompt)),
    signal: AbortSignal.timeout(180_000),
  });
  if (!response.ok) {
    throw new Error(`Codex image request failed (${response.status})`);
  }
  const encoded = imagePayloadFromSse(await readLimitedText(response));
  if (encoded.length > MAX_PNG_BYTES * 2) throw new Error("generated image exceeded the size limit");
  const png = decodeBase64Image(encoded);
  validatePng(png);
  return png;
}

function decodeBase64Image(encoded) {
  if (typeof encoded !== "string" || encoded.length === 0 || encoded.length % 4 !== 0) {
    throw new Error("ChatGPT returned malformed image data");
  }
  let contentLength = encoded.length;
  if (encoded.endsWith("==")) contentLength -= 2;
  else if (encoded.endsWith("=")) contentLength -= 1;
  for (let index = 0; index < encoded.length; index += 1) {
    const code = encoded.charCodeAt(index);
    const base64 = (code >= 65 && code <= 90)
      || (code >= 97 && code <= 122)
      || (code >= 48 && code <= 57)
      || code === 43
      || code === 47;
    if (index < contentLength ? !base64 : code !== 61) {
      throw new Error("ChatGPT returned malformed image data");
    }
  }
  return Buffer.from(encoded, "base64");
}

function crc32(buffer) {
  let value = 0xffffffff;
  for (const byte of buffer) {
    value ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      value = (value >>> 1) ^ (0xedb88320 & -(value & 1));
    }
  }
  return (value ^ 0xffffffff) >>> 0;
}

function validatePng(png) {
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
  if (png.length < 45 || png.length > MAX_PNG_BYTES || !png.subarray(0, 8).equals(signature)) {
    throw new Error("ChatGPT returned an invalid PNG");
  }
  let offset = 8;
  let ihdr = false;
  let idat = false;
  let iend = false;
  while (offset + 12 <= png.length) {
    const length = png.readUInt32BE(offset);
    const type = png.subarray(offset + 4, offset + 8).toString("ascii");
    const dataStart = offset + 8;
    const dataEnd = dataStart + length;
    const crcEnd = dataEnd + 4;
    if (dataEnd < dataStart || crcEnd > png.length) throw new Error("PNG chunk exceeds file bounds");
    const expected = png.readUInt32BE(dataEnd);
    const actual = crc32(png.subarray(offset + 4, dataEnd));
    if (expected !== actual) throw new Error("PNG chunk checksum failed");
    if (!ihdr) {
      if (type !== "IHDR" || length !== 13) throw new Error("PNG has no valid IHDR");
      const width = png.readUInt32BE(dataStart);
      const height = png.readUInt32BE(dataStart + 4);
      const bitDepth = png[dataStart + 8];
      const colorType = png[dataStart + 9];
      const compression = png[dataStart + 10];
      const filter = png[dataStart + 11];
      const interlace = png[dataStart + 12];
      if (width < 1 || height < 1 || width > 4096 || height > 4096 || width * height > 16_777_216) {
        throw new Error("generated image dimensions are unsafe");
      }
      const validDepths = {
        0: new Set([1, 2, 4, 8, 16]),
        2: new Set([8, 16]),
        3: new Set([1, 2, 4, 8]),
        4: new Set([8, 16]),
        6: new Set([8, 16]),
      };
      if (
        !validDepths[colorType]?.has(bitDepth) ||
        compression !== 0 ||
        filter !== 0 ||
        ![0, 1].includes(interlace)
      ) {
        throw new Error("PNG IHDR uses unsupported encoding");
      }
      ihdr = true;
    }
    if (type === "IDAT") idat = true;
    if (type === "IEND") {
      if (length !== 0 || crcEnd !== png.length) throw new Error("PNG has trailing or malformed data");
      iend = true;
      break;
    }
    offset = crcEnd;
  }
  if (!ihdr || !idat || !iend) throw new Error("PNG is incomplete");
  return png;
}

function validatedImageSettings() {
  if (!IMAGE_MODELS.has(IMAGE_MODEL)) throw new Error("unsupported Paper Agent image model");
  if (!IMAGE_SIZES.has(IMAGE_SIZE)) throw new Error("unsupported Paper Agent image size");
  if (!IMAGE_QUALITIES.has(IMAGE_QUALITY)) throw new Error("unsupported Paper Agent image quality");
  if (!/^gpt-5\.6-(?:sol|terra|luna)$/.test(RESPONSES_MODEL)) {
    throw new Error("unsupported Paper Agent image routing model");
  }
  return { model: IMAGE_MODEL, size: IMAGE_SIZE, quality: IMAGE_QUALITY };
}

function buildRequestBody(prompt) {
  const image = validatedImageSettings();
  return {
    model: RESPONSES_MODEL,
    input: [{ role: "user", content: [{ type: "input_text", text: prompt }] }],
    instructions: "Generate the requested image. Do not answer with text.",
    tools: [
      {
        type: "image_generation",
        model: image.model,
        size: image.size,
        quality: image.quality,
        output_format: "png",
        background: "opaque",
      },
    ],
    tool_choice: { type: "image_generation" },
    stream: true,
    store: false,
  };
}

async function writePng(path, png) {
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  try {
    await lstat(path);
    throw new Error("refusing to overwrite an existing image artifact");
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
  const temporary = `${path}.tmp-${randomUUID()}`;
  try {
    await writeFile(temporary, png, { mode: 0o600, flag: "wx" });
    await rename(temporary, path);
    const saved = await stat(path);
    if (!saved.isFile() || saved.size !== png.length) {
      throw new Error("generated PNG was not saved completely");
    }
  } finally {
    await rm(temporary, { force: true });
  }
}

export {
  accountIdFor,
  buildRequestBody,
  decodeBase64Image,
  imagePayloadFromSse,
  validateCredential,
  validatePng,
};

async function main() {
  const output = outputPathFromArgs(process.argv.slice(2));
  const [prompt, credential] = await Promise.all([readPrompt(), usableCredential()]);
  await writePng(output, await generatePng(prompt, credential));
  process.stdout.write(`${output}\n`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((error) => fail(error instanceof Error ? error.message : String(error)));
}

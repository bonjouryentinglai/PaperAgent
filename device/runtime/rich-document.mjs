#!/usr/bin/env node
// SPDX-License-Identifier: MIT
//
// Parse the deliberately small Markdown surface accepted by Paper Agent's
// ::document result. The output manifest contains only allowlisted block kinds
// and hex-encoded UTF-8 data; neither Markdown nor model-provided code is ever
// evaluated by a shell, JavaScript engine, QML, or Xochitl.

import fs from "node:fs";
import { pathToFileURL } from "node:url";

const DOCUMENT_HEADER = "paper-agent-document 1";
const VECTOR_HEADER = "paper-agent-vector 1";
const MAX_DOCUMENT_CHARS = 16_000;
const MAX_BLOCKS = 48;
const MAX_LINES = 2_000;
const MAX_VECTOR_COMMANDS = 256;
const MAX_VECTOR_PATH_POINTS = 64;
const MAX_VECTOR_LABEL_CHARS = 240;
const MAX_VECTOR_ESTIMATED_STROKES = 512;
const MAX_VECTOR_ESTIMATED_POINTS = 24_000;
const MAX_ADAPTIVE_CURVE_POINTS = 257;

function fenceStart(line) {
  const match = line.match(/^ {0,3}(`{3,}|~{3,})\s*([^\s`]*)\s*$/u);
  if (!match) return null;
  return { marker: match[1], info: match[2].toLowerCase() };
}

function fenceEnd(line, marker) {
  const character = marker[0];
  const minimum = marker.length;
  const match = line.match(new RegExp(`^ {0,3}${character}{${minimum},}\\s*$`, "u"));
  return Boolean(match);
}

function tableCells(line) {
  const value = line.trim().replace(/^\|/u, "").replace(/\|$/u, "");
  if (!value.includes("|") && !line.trim().startsWith("|")) return [];
  return value.split("|").map((cell) => cell.trim());
}

function isTableSeparator(line) {
  const cells = tableCells(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/u.test(cell));
}

function isTableStart(lines, index) {
  if (index + 1 >= lines.length) return false;
  const first = tableCells(lines[index]);
  if (first.length === 0) return false;
  if (isTableSeparator(lines[index + 1])) return true;
  const firstLine = lines[index].trim();
  const secondLine = lines[index + 1].trim();
  const second = tableCells(lines[index + 1]);
  return first.length >= 2
    && second.length === first.length
    && firstLine.startsWith("|") && firstLine.endsWith("|")
    && secondLine.startsWith("|") && secondLine.endsWith("|");
}

export function validateVectorBody(body, limits = {}) {
  const maxCommands = limits.maxCommands ?? MAX_VECTOR_COMMANDS;
  const maxLabelChars = limits.maxLabelChars ?? MAX_VECTOR_LABEL_CHARS;
  const maxEstimatedStrokes = limits.maxEstimatedStrokes ?? MAX_VECTOR_ESTIMATED_STROKES;
  const maxEstimatedPoints = limits.maxEstimatedPoints ?? MAX_VECTOR_ESTIMATED_POINTS;
  const labelStrokesPerCharacter = limits.labelStrokesPerCharacter ?? 0;
  const labelPointsPerCharacter = limits.labelPointsPerCharacter ?? 0;
  const lines = body.split("\n").map((line) => line.trim()).filter(Boolean);
  if (lines.shift() !== VECTOR_HEADER) throw new Error(`vector block must start with '${VECTOR_HEADER}'`);
  if (lines.length === 0) throw new Error("vector block contains no commands");
  if (lines.length > maxCommands) {
    throw new Error(`vector block exceeds ${maxCommands} commands`);
  }

  let estimatedStrokes = 0;
  let estimatedPoints = 0;
  const boundedIntegers = (command, values, maximum = 1000, kind = "coordinate") => {
    if (!values.every((value) => /^\d{1,4}$/u.test(value) && Number(value) <= maximum)) {
      throw new Error(`vector command '${command}' has a ${kind} outside 0..=${maximum}`);
    }
    return values.map(Number);
  };
  const boundedSignedIntegers = (command, values, maximum, kind) => {
    if (!values.every((value) => /^-?\d{1,4}$/u.test(value) && Math.abs(Number(value)) <= maximum)) {
      throw new Error(`vector command '${command}' has a ${kind} outside -${maximum}..=${maximum}`);
    }
    return values.map(Number);
  };
  const boundedPath = (command, values, minimum) => {
    const pointCount = values.length / 2;
    if (pointCount < minimum || pointCount > MAX_VECTOR_PATH_POINTS) {
      throw new Error(
        `vector ${command} must contain ${minimum}..=${MAX_VECTOR_PATH_POINTS} points`,
      );
    }
    return pointCount;
  };
  const validateRect = (command, values) => {
    const [x, y, width, height] = values;
    if (width === 0 || height === 0) {
      throw new Error(`vector ${command} width and height must be positive`);
    }
    if (x + width > 1000 || y + height > 1000) {
      throw new Error(`vector ${command} extends outside 0..=1000`);
    }
  };
  const validateEllipse = (command, values) => {
    const [cx, cy, rx, ry = rx] = values;
    if (rx === 0 || ry === 0) throw new Error(`vector ${command} radius must be positive`);
    if (cx - rx < 0 || cy - ry < 0 || cx + rx > 1000 || cy + ry > 1000) {
      throw new Error(`vector ${command} extends outside 0..=1000`);
    }
  };

  for (const line of lines) {
    const fields = line.split(/\s+/u);
    const command = fields[0];
    let coordinates = [];
    let angles = [];
    let signedAngles = [];
    let commandStrokes = 1;
    let commandPoints = 2;
    if (command === "line" || command === "arrow") {
      if (fields.length !== 5) throw new Error(`malformed vector ${command} command`);
      coordinates = fields.slice(1);
      if (command === "arrow") commandStrokes = 3;
    } else if (command === "rect" || command === "box") {
      if (fields.length !== 5) throw new Error(`malformed vector ${command} command`);
      coordinates = fields.slice(1);
      if (command === "box") {
        commandStrokes = 33;
        commandPoints = 70;
      } else {
        commandPoints = 5;
      }
    } else if (command === "rrect") {
      if (fields.length !== 6) throw new Error("malformed vector rrect command");
      coordinates = fields.slice(1);
      commandPoints = 37;
    } else if (command === "circle" || command === "disc") {
      if (fields.length !== 4) throw new Error("malformed vector circle command");
      coordinates = fields.slice(1);
      commandPoints = 49;
      if (command === "disc") {
        commandStrokes = 33;
        commandPoints += 64;
      }
    } else if (command === "ellipse" || command === "fillellipse") {
      if (fields.length !== 5) throw new Error(`malformed vector ${command} command`);
      coordinates = fields.slice(1);
      commandPoints = 49;
      if (command === "fillellipse") {
        commandStrokes = 33;
        commandPoints += 64;
      }
    } else if (command === "polyline" || command === "polygon" || command === "fillpoly") {
      const minimum = command === "polyline" ? 2 : 3;
      if (fields.length < 1 + minimum * 2 || fields.length % 2 !== 1) {
        throw new Error(`malformed vector ${command} command`);
      }
      coordinates = fields.slice(1);
      const pointCount = boundedPath(command, coordinates, minimum);
      commandPoints = pointCount + (command === "polyline" ? 0 : 1);
      if (command === "fillpoly") {
        commandStrokes = 33;
        commandPoints += 64;
      }
    } else if (command === "curve") {
      if (fields.length !== 7 && fields.length !== 9) {
        throw new Error("malformed vector curve command");
      }
      coordinates = fields.slice(1);
      commandPoints = MAX_ADAPTIVE_CURVE_POINTS;
    } else if (command === "arc" || command === "wedge") {
      if (fields.length !== 6) throw new Error(`malformed vector ${command} command`);
      coordinates = fields.slice(1, 4);
      angles = fields.slice(4);
      commandPoints = MAX_ADAPTIVE_CURVE_POINTS;
      if (command === "wedge") {
        commandStrokes = 33;
        commandPoints += MAX_ADAPTIVE_CURVE_POINTS + 1;
      }
    } else if (command === "ellarc") {
      if (fields.length !== 7) throw new Error("malformed vector ellarc command");
      coordinates = fields.slice(1, 5);
      angles = fields.slice(5, 6);
      signedAngles = fields.slice(6);
      commandPoints = MAX_ADAPTIVE_CURVE_POINTS;
    } else if (command === "dot") {
      if (fields.length !== 3) throw new Error("malformed vector dot command");
      coordinates = fields.slice(1);
      commandStrokes = 2;
      commandPoints = 4;
    } else if (["label", "labelleft", "labelright"].includes(command)) {
      if (fields.length < 6) throw new Error("malformed vector label command");
      coordinates = fields.slice(1, 5);
      const labelCharacters = [...fields.slice(5).join(" ")].length;
      if (labelCharacters > maxLabelChars) {
        throw new Error(`vector label exceeds ${maxLabelChars} characters`);
      }
      commandStrokes = Math.max(1, labelCharacters * labelStrokesPerCharacter);
      commandPoints = Math.max(2, labelCharacters * labelPointsPerCharacter);
    } else {
      throw new Error(`unsupported vector command '${command}'`);
    }

    const values = boundedIntegers(command, coordinates);
    const angleValues = boundedIntegers(command, angles, 360, "angle");
    const signedAngleValues = boundedSignedIntegers(command, signedAngles, 360, "sweep angle");
    if (["rect", "box", "rrect", "label"].includes(command)) {
      validateRect(command, values);
    }
    if (command === "rrect") {
      const radius = values[4];
      if (radius === 0 || radius > values[2] / 2 || radius > values[3] / 2) {
        throw new Error("vector rrect radius must be positive and no larger than half either side");
      }
    }
    if (["circle", "disc", "arc", "wedge"].includes(command)) {
      validateEllipse(command, values.slice(0, 3));
    } else if (["ellipse", "fillellipse", "ellarc"].includes(command)) {
      validateEllipse(command, values);
    }
    if ((command === "arc" || command === "wedge") && angleValues[0] === angleValues[1]) {
      throw new Error(`vector ${command} angle sweep must be nonzero`);
    }
    if (command === "ellarc" && signedAngleValues[0] === 0) {
      throw new Error("vector ellarc sweep angle must be nonzero");
    }

    estimatedStrokes += commandStrokes;
    estimatedPoints += commandPoints;
    if (estimatedStrokes > maxEstimatedStrokes) {
      throw new Error(`vector expands past ${maxEstimatedStrokes} strokes`);
    }
    if (estimatedPoints > maxEstimatedPoints) {
      throw new Error(`vector expands past ${maxEstimatedPoints} source points`);
    }
  }
  limits.onStats?.({
    commands: lines.length,
    estimatedStrokes,
    estimatedPoints,
  });
  return body.trim();
}

export function parseRichDocument(markdown) {
  if (typeof markdown !== "string") throw new Error("document body must be text");
  const normalized = markdown.replace(/\r\n?/gu, "\n").trim();
  if (!normalized) throw new Error("document contains no content");
  if (normalized.includes("\0")) throw new Error("document contains a NUL byte");
  if ([...normalized].length > MAX_DOCUMENT_CHARS) {
    throw new Error(`document exceeds ${MAX_DOCUMENT_CHARS} characters`);
  }
  const lines = normalized.split("\n");
  if (lines.length > MAX_LINES) throw new Error(`document exceeds ${MAX_LINES} lines`);

  const blocks = [];
  let rich = [];
  const pushBlock = (kind, body) => {
    const clean = body.replace(/^\n+|\n+$/gu, "");
    if (!clean) return;
    if (blocks.length >= MAX_BLOCKS) throw new Error(`document exceeds ${MAX_BLOCKS} blocks`);
    blocks.push({ kind, body: clean });
  };
  const flushRich = () => {
    pushBlock("rich", rich.join("\n"));
    rich = [];
  };

  for (let index = 0; index < lines.length;) {
    const opening = fenceStart(lines[index]);
    if (opening) {
      const fenced = [lines[index]];
      const body = [];
      let cursor = index + 1;
      for (; cursor < lines.length && !fenceEnd(lines[cursor], opening.marker); cursor += 1) {
        fenced.push(lines[cursor]);
        body.push(lines[cursor]);
      }
      if (cursor < lines.length) fenced.push(lines[cursor]);
      const vector = ["paper-agent-vector", "vector", "draw"].includes(opening.info);
      if (vector) {
        if (cursor >= lines.length) throw new Error("unterminated vector fence");
        flushRich();
        pushBlock("vector", validateVectorBody(body.join("\n")));
      } else {
        rich.push(...fenced);
      }
      index = Math.min(cursor + 1, lines.length);
      continue;
    }

    if (isTableStart(lines, index)) {
      flushRich();
      const columnCount = tableCells(lines[index]).length;
      const table = [];
      while (index < lines.length && lines[index].trim()) {
        const cells = tableCells(lines[index]);
        if (cells.length !== columnCount && !isTableSeparator(lines[index])) break;
        table.push(lines[index]);
        index += 1;
      }
      pushBlock("table", table.join("\n"));
      continue;
    }

    rich.push(lines[index]);
    index += 1;
  }
  flushRich();
  if (blocks.length === 0) throw new Error("document contains no renderable blocks");
  return blocks;
}

export function serializeRichDocument(blocks) {
  if (!Array.isArray(blocks) || blocks.length === 0 || blocks.length > MAX_BLOCKS) {
    throw new Error("document block list is empty or too large");
  }
  const records = [DOCUMENT_HEADER];
  for (const block of blocks) {
    if (!block || !["rich", "table", "vector"].includes(block.kind) || typeof block.body !== "string") {
      throw new Error("document contains an invalid block");
    }
    const encoded = Buffer.from(block.body, "utf8").toString("hex");
    if (!encoded) throw new Error("document contains an empty block");
    records.push(`block ${block.kind} ${encoded}`);
  }
  return `${records.join("\n")}\n`;
}

export function compileRichDocument(markdown) {
  return serializeRichDocument(parseRichDocument(markdown));
}

async function cli() {
  const args = process.argv.slice(2);
  if (args.length !== 3 || args[0] !== "--compile") {
    console.error("usage: rich-document.mjs --compile INPUT.md OUTPUT.document");
    process.exitCode = 2;
    return;
  }
  const compiled = compileRichDocument(fs.readFileSync(args[1], "utf8"));
  fs.writeFileSync(args[2], compiled, { mode: 0o600 });
}

if (import.meta.url === pathToFileURL(process.argv[1] || "").href) {
  await cli();
}

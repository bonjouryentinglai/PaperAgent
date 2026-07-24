#!/usr/bin/env node
// SPDX-License-Identifier: MIT
//
// Validate Paper Agent Scene v1 and compile it into the existing bounded
// native-vector language. Scene is semantic JSON from the model; the compiler
// is deterministic and never executes model-provided text or code.

import { validateVectorBody } from "./rich-document.mjs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SCENE_VERSION = 1;
const MIN_CANVAS = 48;
const MAX_CANVAS = 4_000;
const MAX_OBJECTS = 192;
const MAX_CELLS = 256;
const MAX_TOTAL_CELLS = 512;
const MAX_TEXT_CHARS = 1_200;
const MAX_TOTAL_TEXT_CHARS = 16_000;
const MAX_LABEL_CHARS = MAX_TEXT_CHARS;
const MAX_POINTS = 64;
const MAX_TOTAL_PATH_POINTS = 4_096;
const MAX_COMPILED_COMMANDS = 512;
const MAX_COMMANDS_PER_RUN = 96;
const MAX_RUN_ESTIMATED_STROKES = 4_096;
const MAX_RUN_ESTIMATED_POINTS = 96_000;
const MAX_TOTAL_ESTIMATED_STROKES = 8_192;
const MAX_TOTAL_ESTIMATED_POINTS = 192_000;
const SCENE_LABEL_STROKES_PER_CHARACTER = 8;
const SCENE_LABEL_POINTS_PER_CHARACTER = 160;
const PROSE_FLOW_MIN_CHARACTERS = 600;
const PROSE_FLOW_MIN_ENTRIES = 8;
const PROSE_FLOW_LONG_ENTRY_CHARACTERS = 320;
const COLORS = new Set(["black", "gray", "blue", "red", "green", "yellow", "cyan", "magenta"]);
const WIDTHS = new Set(["thin", "medium", "thick"]);
const ALIGNS = new Set(["left", "center", "right"]);
const LAYOUTS = new Set(["auto", "flow", "spatial"]);
const OBJECT_TYPES = new Set([
  "text", "line", "arrow", "rect", "ellipse", "circle", "polyline", "grid",
]);

const DEFAULT_STYLE = Object.freeze({ color: "black", width: "medium" });

function own(value, key) {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function object(value, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  return value;
}

function exactKeys(value, allowed, label) {
  for (const key of Object.keys(value)) {
    if (!allowed.has(key)) throw new Error(`${label} contains unsupported field '${key}'`);
  }
}

function integer(value, label, minimum, maximum) {
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) {
    throw new Error(`${label} must be an integer within ${minimum}..=${maximum}`);
  }
  return value;
}

function boundedText(value, label, maximum = MAX_TEXT_CHARS) {
  if (typeof value !== "string") throw new Error(`${label} must be text`);
  const normalized = value.replace(/\r\n?/gu, "\n").trim();
  if (!normalized || normalized.includes("\0") || [...normalized].length > maximum) {
    throw new Error(`${label} must contain 1..=${maximum} safe characters`);
  }
  return normalized;
}

function enumValue(value, allowed, fallback, label) {
  const result = value === undefined ? fallback : value;
  if (!allowed.has(result)) throw new Error(`${label} is unsupported`);
  return result;
}

function styleFor(value, fallback = DEFAULT_STYLE) {
  return {
    color: enumValue(value.color, COLORS, fallback.color, "scene color"),
    width: enumValue(value.strokeWidth, WIDTHS, fallback.width, "scene strokeWidth"),
  };
}

function sceneTextEntries(objects) {
  const entries = [];
  for (const item of objects) {
    if (item.type === "text") {
      entries.push({
        x: item.x,
        y: item.y,
        width: item.width,
        height: item.height,
        text: item.text,
        style: item.style,
        group: item.group,
      });
      continue;
    }
    if (item.type !== "grid") continue;
    for (const cell of item.cells) {
      entries.push({
        x: item.x + item.width * cell.column / item.columns,
        y: item.y + item.height * cell.row / item.rows,
        width: item.width / item.columns,
        height: item.height / item.rows,
        text: cell.text,
        style: { color: cell.color, width: "medium" },
        group: item.group,
      });
    }
  }
  return entries.sort((left, right) => left.y - right.y || left.x - right.x);
}

function leadingOrdinal(text) {
  const match = text.match(/^\s*(\d{1,3})\s*[.)、．]\s*/u);
  return match ? Number(match[1]) : null;
}

function orderFlowEntries(entries, canvas) {
  const numbered = entries
    .map((entry) => ({ entry, ordinal: leadingOrdinal(entry.text) }))
    .filter((item) => item.ordinal !== null);
  if (numbered.length < 3 || new Set(numbered.map((item) => item.ordinal)).size !== numbered.length) {
    return entries;
  }

  const firstNumberY = Math.min(...numbered.map((item) => item.entry.y));
  const headers = entries
    .filter((entry) => leadingOrdinal(entry.text) === null && entry.y < firstNumberY)
    .sort((left, right) => left.y - right.y || left.x - right.x);
  const continuations = new Map(numbered.map((item) => [item.entry, []]));
  const unattached = [];
  for (const entry of entries) {
    if (leadingOrdinal(entry.text) !== null || headers.includes(entry)) continue;
    const centerX = entry.x + entry.width / 2;
    const candidate = numbered
      .filter((item) => {
        const itemCenterX = item.entry.x + item.entry.width / 2;
        return item.entry.y <= entry.y
          && Math.abs(itemCenterX - centerX) <= canvas.width * 0.3;
      })
      .sort((left, right) => right.entry.y - left.entry.y)[0];
    if (candidate) continuations.get(candidate.entry).push(entry);
    else unattached.push(entry);
  }

  const ordered = [...headers];
  for (const item of numbered.sort((left, right) => left.ordinal - right.ordinal)) {
    ordered.push(item.entry);
    ordered.push(...continuations.get(item.entry).sort(
      (left, right) => left.y - right.y || left.x - right.x,
    ));
  }
  ordered.push(...unattached.sort((left, right) => left.y - right.y || left.x - right.x));
  return ordered;
}

function isProseDominated(entries, totalTextChars) {
  if (totalTextChars < PROSE_FLOW_MIN_CHARACTERS || entries.length === 0) return false;
  const punctuationCount = entries.reduce(
    (sum, entry) => sum + (entry.text.match(/[。！？；.!?;]/gu)?.length ?? 0),
    0,
  );
  return entries.length >= PROSE_FLOW_MIN_ENTRIES
    || punctuationCount >= 6
    || entries.some((entry) => [...entry.text].length >= PROSE_FLOW_LONG_ENTRY_CHARACTERS);
}

function textBoxInsideContainingRect(item, objects) {
  if (item.type !== "text" || [...item.text].length <= 24) return item;
  const itemRight = item.x + item.width;
  const itemBottom = item.y + item.height;
  const itemArea = item.width * item.height;
  const container = objects
    .filter((candidate) => candidate.type === "rect"
      && !candidate.filled
      && candidate.x <= item.x
      && candidate.y <= item.y
      && candidate.x + candidate.width >= itemRight
      && candidate.y + candidate.height >= itemBottom
      && candidate.width * candidate.height <= itemArea * 12)
    .sort((left, right) => left.width * left.height - right.width * right.height)[0];
  if (!container) return item;
  const padding = Math.max(
    8,
    Math.round(Math.min(container.width, container.height) * 0.08),
  );
  if (container.width <= padding * 2 || container.height <= padding * 2) return item;
  return {
    ...item,
    x: container.x + padding,
    y: container.y + padding,
    width: container.width - padding * 2,
    height: container.height - padding * 2,
  };
}

function validateRect(value, canvas, label) {
  const x = integer(value.x, `${label}.x`, 0, canvas.width);
  const y = integer(value.y, `${label}.y`, 0, canvas.height);
  const width = integer(value.width, `${label}.width`, 1, canvas.width);
  const height = integer(value.height, `${label}.height`, 1, canvas.height);
  if (x + width > canvas.width || y + height > canvas.height) {
    throw new Error(`${label} extends outside the scene canvas`);
  }
  return { x, y, width, height };
}

function validatePoint(value, canvas, label) {
  const point = object(value, label);
  exactKeys(point, new Set(["x", "y"]), label);
  return {
    x: integer(point.x, `${label}.x`, 0, canvas.width),
    y: integer(point.y, `${label}.y`, 0, canvas.height),
  };
}

function common(value, fallbackStyle) {
  return {
    style: styleFor(value, fallbackStyle),
    group: value.group === undefined
      ? null
      : boundedText(value.group, "scene group", 48),
  };
}

function validateSceneObject(raw, canvas, index, fallbackStyle) {
  const value = object(raw, `scene object ${index}`);
  if (!OBJECT_TYPES.has(value.type)) throw new Error(`scene object ${index} has unsupported type`);
  const shared = new Set(["type", "color", "strokeWidth", "group"]);
  const label = `scene ${value.type} ${index}`;
  const base = common(value, fallbackStyle);

  if (value.type === "text") {
    exactKeys(value, new Set([...shared, "x", "y", "width", "height", "text", "align"]), label);
    return {
      type: "text",
      ...validateRect(value, canvas, label),
      text: boundedText(value.text, `${label}.text`),
      align: enumValue(value.align, ALIGNS, "center", `${label}.align`),
      ...base,
    };
  }

  if (value.type === "line" || value.type === "arrow") {
    exactKeys(value, new Set([...shared, "x1", "y1", "x2", "y2"]), label);
    return {
      type: value.type,
      x1: integer(value.x1, `${label}.x1`, 0, canvas.width),
      y1: integer(value.y1, `${label}.y1`, 0, canvas.height),
      x2: integer(value.x2, `${label}.x2`, 0, canvas.width),
      y2: integer(value.y2, `${label}.y2`, 0, canvas.height),
      ...base,
    };
  }

  if (value.type === "rect") {
    exactKeys(value, new Set([...shared, "x", "y", "width", "height", "radius", "filled"]), label);
    const rect = validateRect(value, canvas, label);
    const radius = value.radius === undefined
      ? 0
      : integer(value.radius, `${label}.radius`, 0, Math.floor(Math.min(rect.width, rect.height) / 2));
    if (value.filled !== undefined && typeof value.filled !== "boolean") {
      throw new Error(`${label}.filled must be boolean`);
    }
    return { type: "rect", ...rect, radius, filled: value.filled === true, ...base };
  }

  if (value.type === "ellipse" || value.type === "circle") {
    const keys = value.type === "circle"
      ? new Set([...shared, "cx", "cy", "radius", "filled"])
      : new Set([...shared, "cx", "cy", "rx", "ry", "filled"]);
    exactKeys(value, keys, label);
    const cx = integer(value.cx, `${label}.cx`, 0, canvas.width);
    const cy = integer(value.cy, `${label}.cy`, 0, canvas.height);
    const rx = integer(value.type === "circle" ? value.radius : value.rx, `${label}.rx`, 1, canvas.width);
    const ry = integer(value.type === "circle" ? value.radius : value.ry, `${label}.ry`, 1, canvas.height);
    if (cx - rx < 0 || cy - ry < 0 || cx + rx > canvas.width || cy + ry > canvas.height) {
      throw new Error(`${label} extends outside the scene canvas`);
    }
    if (value.filled !== undefined && typeof value.filled !== "boolean") {
      throw new Error(`${label}.filled must be boolean`);
    }
    return { type: value.type, cx, cy, rx, ry, filled: value.filled === true, ...base };
  }

  if (value.type === "polyline") {
    exactKeys(value, new Set([...shared, "points", "closed", "filled"]), label);
    if (!Array.isArray(value.points) || value.points.length < 2 || value.points.length > MAX_POINTS) {
      throw new Error(`${label}.points must contain 2..=${MAX_POINTS} points`);
    }
    if (value.closed !== undefined && typeof value.closed !== "boolean") {
      throw new Error(`${label}.closed must be boolean`);
    }
    if (value.filled !== undefined && typeof value.filled !== "boolean") {
      throw new Error(`${label}.filled must be boolean`);
    }
    const closed = value.closed === true || value.filled === true;
    if (closed && value.points.length < 3) throw new Error(`${label} needs three points when closed`);
    return {
      type: "polyline",
      points: value.points.map((point, pointIndex) => validatePoint(point, canvas, `${label}.points[${pointIndex}]`)),
      closed,
      filled: value.filled === true,
      ...base,
    };
  }

  exactKeys(value, new Set([
    ...shared, "x", "y", "width", "height", "rows", "columns", "majorEvery",
    "majorStrokeWidth", "cells",
  ]), label);
  const rect = validateRect(value, canvas, label);
  const rows = integer(value.rows, `${label}.rows`, 1, 32);
  const columns = integer(value.columns, `${label}.columns`, 1, 32);
  const majorEvery = value.majorEvery === undefined
    ? 1
    : integer(value.majorEvery, `${label}.majorEvery`, 1, Math.max(rows, columns));
  const majorWidth = enumValue(value.majorStrokeWidth, WIDTHS, "thick", `${label}.majorStrokeWidth`);
  const cells = value.cells === undefined ? [] : value.cells;
  if (!Array.isArray(cells) || cells.length > Math.min(MAX_CELLS, rows * columns)) {
    throw new Error(`${label}.cells exceeds its bounded grid`);
  }
  const seen = new Set();
  const normalizedCells = cells.map((rawCell, cellIndex) => {
    const cell = object(rawCell, `${label}.cells[${cellIndex}]`);
    exactKeys(cell, new Set(["row", "column", "text", "color"]), `${label}.cells[${cellIndex}]`);
    const row = integer(cell.row, `${label}.cells[${cellIndex}].row`, 0, rows - 1);
    const column = integer(cell.column, `${label}.cells[${cellIndex}].column`, 0, columns - 1);
    const key = `${row}:${column}`;
    if (seen.has(key)) throw new Error(`${label} contains duplicate cell ${key}`);
    seen.add(key);
    return {
      row,
      column,
      text: boundedText(cell.text, `${label}.cells[${cellIndex}].text`, 64),
      color: enumValue(cell.color, COLORS, base.style.color, `${label}.cells[${cellIndex}].color`),
    };
  });
  return { type: "grid", ...rect, rows, columns, majorEvery, majorWidth, cells: normalizedCells, ...base };
}

export function validateSceneToolCall(raw, action = "ai") {
  const value = object(raw, "move_render_scene arguments");
  exactKeys(value, new Set(["version", "canvas", "objects", "background", "layout"]), "move_render_scene arguments");
  if (value.version !== SCENE_VERSION) throw new Error(`scene version must be ${SCENE_VERSION}`);
  const canvasRaw = object(value.canvas, "scene canvas");
  exactKeys(canvasRaw, new Set(["width", "height"]), "scene canvas");
  const canvas = {
    width: integer(canvasRaw.width, "scene canvas width", MIN_CANVAS, MAX_CANVAS),
    height: integer(canvasRaw.height, "scene canvas height", MIN_CANVAS, MAX_CANVAS),
  };
  if (!Array.isArray(value.objects) || value.objects.length < 1 || value.objects.length > MAX_OBJECTS) {
    throw new Error(`scene objects must contain 1..=${MAX_OBJECTS} items`);
  }
  const background = value.background === undefined ? "transparent" : value.background;
  if (background !== "transparent") throw new Error("only a transparent scene background is supported");
  const objects = value.objects.map((item, index) => validateSceneObject(item, canvas, index, DEFAULT_STYLE));
  const totalCells = objects.reduce(
    (sum, item) => sum + (item.type === "grid" ? item.cells.length : 0),
    0,
  );
  if (totalCells > MAX_TOTAL_CELLS) {
    throw new Error(`scene contains more than ${MAX_TOTAL_CELLS} populated grid cells`);
  }
  const totalPathPoints = objects.reduce(
    (sum, item) => sum + (item.type === "polyline" ? item.points.length : 0),
    0,
  );
  if (totalPathPoints > MAX_TOTAL_PATH_POINTS) {
    throw new Error(`scene contains more than ${MAX_TOTAL_PATH_POINTS} polyline points`);
  }
  const totalTextChars = objects.reduce((sum, item) => {
    if (item.type === "text") return sum + [...item.text].length;
    if (item.type === "grid") {
      return sum + item.cells.reduce((cellSum, cell) => cellSum + [...cell.text].length, 0);
    }
    return sum;
  }, 0);
  if (totalTextChars > MAX_TOTAL_TEXT_CHARS) {
    throw new Error(`scene contains more than ${MAX_TOTAL_TEXT_CHARS} text characters`);
  }
  const requestedLayout = enumValue(value.layout, LAYOUTS, "auto", "scene layout");
  const textEntries = sceneTextEntries(objects);
  const hasText = textEntries.length > 0;
  if (requestedLayout === "flow" && !hasText) {
    throw new Error("flow Scene layout requires text content");
  }
  const flowCompatible = hasText
    && objects.every((item) => item.type === "text"
      || (item.type === "line" && item.y1 === item.y2));
  const layout = action === "beautify"
    ? "spatial"
    : (flowCompatible
        || isProseDominated(textEntries, totalTextChars)
      ? "flow"
      : "spatial");
  if (action === "beautify" && objects.some((item) => item.type === "text" && item.text.length === 0)) {
    throw new Error("Beautify cannot emit empty text");
  }
  return { version: SCENE_VERSION, canvas, background, layout, objects };
}

function makeMapper(scene, target) {
  const margin = 10;
  const innerWidth = Math.max(1, target.width - margin * 2 - 1);
  const innerHeight = Math.max(1, target.height - margin * 2 - 1);
  const scale = Math.min(innerWidth / scene.canvas.width, innerHeight / scene.canvas.height);
  const offsetX = (innerWidth - scene.canvas.width * scale) / 2;
  const offsetY = (innerHeight - scene.canvas.height * scale) / 2;
  const axis = (value, sceneMaximum, innerMaximum, offset) => Math.max(0, Math.min(1000,
    Math.round((offset + value * scale) / innerMaximum * 1000),
  ));
  return {
    x: (value) => axis(value, scene.canvas.width, innerWidth, offsetX),
    y: (value) => axis(value, scene.canvas.height, innerHeight, offsetY),
    width: (x, width) => {
      const start = axis(x, scene.canvas.width, innerWidth, offsetX);
      return Math.max(1, axis(x + width, scene.canvas.width, innerWidth, offsetX) - start);
    },
    height: (y, height) => {
      const start = axis(y, scene.canvas.height, innerHeight, offsetY);
      return Math.max(1, axis(y + height, scene.canvas.height, innerHeight, offsetY) - start);
    },
  };
}

function styleKey(style) {
  return `${style.color}:${style.width}`;
}

function safeLabel(text) {
  return text.replace(/\s+/gu, " ").trim();
}

function pushCommand(runs, style, command, group = null) {
  const key = styleKey(style);
  const existing = runs.find(
    (run) => run.key === key && run.commands.length < MAX_COMMANDS_PER_RUN,
  );
  if (existing) {
    existing.commands.push(command);
    if (group) existing.groups.add(group);
  } else {
    runs.push({ key, style, commands: [command], groups: new Set(group ? [group] : []) });
  }
}

function lineStyle(base, width) {
  return { color: base.color, width };
}

function compileText(runs, item, map) {
  const lines = item.text.split("\n").map((line) => line.trim()).filter(Boolean);
  if (lines.length === 0) return;
  const lineHeight = item.height / lines.length;
  for (let index = 0; index < lines.length; index += 1) {
    const text = safeLabel(lines[index]);
    if (!text) continue;
    const y = item.y + lineHeight * index;
    const x0 = map.x(item.x);
    const y0 = map.y(Math.round(y));
    const width = map.width(item.x, item.width);
    const height = map.height(Math.round(y), Math.max(1, Math.round(lineHeight)));
    if ([...text].length > MAX_LABEL_CHARS) throw new Error("scene text line exceeds native label limit");
    // Keep alignment semantic until the native renderer performs final glyph
    // fitting. Text remains one explicit source line per label command.
    const command = item.align === "left" ? "labelleft" : (item.align === "right" ? "labelright" : "label");
    pushCommand(runs, item.style, `${command} ${x0} ${y0} ${width} ${height} ${text}`, item.group);
  }
}

function compileObject(runs, item, map) {
  if (item.type === "text") return compileText(runs, item, map);
  if (item.type === "line" || item.type === "arrow") {
    pushCommand(runs, item.style, `${item.type} ${map.x(item.x1)} ${map.y(item.y1)} ${map.x(item.x2)} ${map.y(item.y2)}`, item.group);
    return;
  }
  if (item.type === "rect") {
    const x = map.x(item.x);
    const y = map.y(item.y);
    const width = map.width(item.x, item.width);
    const height = map.height(item.y, item.height);
    if (item.filled) pushCommand(runs, item.style, `box ${x} ${y} ${width} ${height}`, item.group);
    else if (item.radius > 0) {
      const radius = Math.max(1, Math.min(
        map.width(item.x, item.radius), map.height(item.y, item.radius),
        Math.floor(width / 2), Math.floor(height / 2),
      ));
      pushCommand(runs, item.style, `rrect ${x} ${y} ${width} ${height} ${radius}`, item.group);
    } else pushCommand(runs, item.style, `rect ${x} ${y} ${width} ${height}`, item.group);
    return;
  }
  if (item.type === "ellipse" || item.type === "circle") {
    const cx = map.x(item.cx);
    const cy = map.y(item.cy);
    const rx = Math.max(1, Math.abs(map.x(item.cx + item.rx) - cx));
    const ry = Math.max(1, Math.abs(map.y(item.cy + item.ry) - cy));
    if (item.type === "circle" && Math.abs(rx - ry) <= 1) {
      pushCommand(runs, item.style, `${item.filled ? "disc" : "circle"} ${cx} ${cy} ${Math.min(rx, ry)}`, item.group);
    } else {
      pushCommand(runs, item.style, `${item.filled ? "fillellipse" : "ellipse"} ${cx} ${cy} ${rx} ${ry}`, item.group);
    }
    return;
  }
  if (item.type === "polyline") {
    const points = item.points.flatMap((point) => [map.x(point.x), map.y(point.y)]).join(" ");
    const command = item.filled ? "fillpoly" : (item.closed ? "polygon" : "polyline");
    pushCommand(runs, item.style, `${command} ${points}`, item.group);
    return;
  }
  const left = map.x(item.x);
  const top = map.y(item.y);
  const right = map.x(item.x + item.width);
  const bottom = map.y(item.y + item.height);
  for (let column = 0; column <= item.columns; column += 1) {
    const x = map.x(Math.round(item.x + item.width * column / item.columns));
    const major = column === 0 || column === item.columns || column % item.majorEvery === 0;
    pushCommand(runs, lineStyle(item.style, major ? item.majorWidth : item.style.width), `line ${x} ${top} ${x} ${bottom}`, item.group);
  }
  for (let row = 0; row <= item.rows; row += 1) {
    const y = map.y(Math.round(item.y + item.height * row / item.rows));
    const major = row === 0 || row === item.rows || row % item.majorEvery === 0;
    pushCommand(runs, lineStyle(item.style, major ? item.majorWidth : item.style.width), `line ${left} ${y} ${right} ${y}`, item.group);
  }
  for (const cell of item.cells) {
    const cellX = item.x + item.width * cell.column / item.columns;
    const cellY = item.y + item.height * cell.row / item.rows;
    const textStyle = { color: cell.color, width: "medium" };
    compileText(runs, {
      type: "text",
      x: Math.round(cellX),
      y: Math.round(cellY),
      width: Math.max(1, Math.round(item.width / item.columns)),
      height: Math.max(1, Math.round(item.height / item.rows)),
      text: cell.text,
      align: "center",
      style: textStyle,
      group: item.group,
    }, map);
  }
}

export function compileScene(raw, target, action = "ai") {
  const scene = validateSceneToolCall(raw, action);
  if (!target || !Number.isSafeInteger(target.width) || !Number.isSafeInteger(target.height)
      || target.width < 48 || target.height < 48) {
    throw new Error("scene target must be at least 48 x 48 pixels");
  }
  if (scene.layout === "flow") {
    const textItems = orderFlowEntries(sceneTextEntries(scene.objects), scene.canvas);
    const item = textItems[0];
    return [{
      kind: "bodyText",
      style: item.style,
      body: textItems.map((entry) => entry.text).join("\n"),
      groups: [...new Set(textItems.map((entry) => entry.group).filter(Boolean))],
    }];
  }
  if (action === "beautify" && scene.objects.length === 1 && scene.objects[0].type === "text") {
    const item = scene.objects[0];
    return [{
      kind: "beautifyText",
      style: item.style,
      body: item.text,
      groups: item.group ? [item.group] : [],
    }];
  }

  const map = makeMapper(scene, target);
  const runs = [];
  for (const item of scene.objects) {
    compileObject(runs, textBoxInsideContainingRect(item, scene.objects), map);
  }
  if (runs.length === 0) throw new Error("scene contains no drawable content");
  const commandCount = runs.reduce((sum, run) => sum + run.commands.length, 0);
  if (commandCount > MAX_COMPILED_COMMANDS) {
    throw new Error(`scene expands past ${MAX_COMPILED_COMMANDS} native commands`);
  }
  if (runs.length > 96) throw new Error("scene expands into too many styled runs");
  let totalEstimatedStrokes = 0;
  let totalEstimatedPoints = 0;
  const compiled = runs.map((run) => {
    const body = validateVectorBody(`paper-agent-vector 1\n${run.commands.join("\n")}`, {
      maxCommands: MAX_COMMANDS_PER_RUN,
      maxLabelChars: MAX_LABEL_CHARS,
      maxEstimatedStrokes: MAX_RUN_ESTIMATED_STROKES,
      maxEstimatedPoints: MAX_RUN_ESTIMATED_POINTS,
      labelStrokesPerCharacter: SCENE_LABEL_STROKES_PER_CHARACTER,
      labelPointsPerCharacter: SCENE_LABEL_POINTS_PER_CHARACTER,
      onStats(stats) {
        totalEstimatedStrokes += stats.estimatedStrokes;
        totalEstimatedPoints += stats.estimatedPoints;
      },
    });
    return { kind: "vector", style: run.style, body, groups: [...run.groups] };
  });
  if (totalEstimatedStrokes > MAX_TOTAL_ESTIMATED_STROKES) {
    throw new Error(`scene expands past ${MAX_TOTAL_ESTIMATED_STROKES} estimated strokes`);
  }
  if (totalEstimatedPoints > MAX_TOTAL_ESTIMATED_POINTS) {
    throw new Error(`scene expands past ${MAX_TOTAL_ESTIMATED_POINTS} estimated source points`);
  }
  return compiled;
}

export const SCENE_LIMITS = Object.freeze({
  version: SCENE_VERSION,
  minCanvas: MIN_CANVAS,
  maxCanvas: MAX_CANVAS,
  maxObjects: MAX_OBJECTS,
  maxCells: MAX_CELLS,
  maxTotalCells: MAX_TOTAL_CELLS,
  maxTotalTextChars: MAX_TOTAL_TEXT_CHARS,
  maxTotalPathPoints: MAX_TOTAL_PATH_POINTS,
  maxCompiledCommands: MAX_COMPILED_COMMANDS,
  maxCommandsPerRun: MAX_COMMANDS_PER_RUN,
  maxTotalEstimatedStrokes: MAX_TOTAL_ESTIMATED_STROKES,
  maxTotalEstimatedPoints: MAX_TOTAL_ESTIMATED_POINTS,
  colors: [...COLORS],
  widths: [...WIDTHS],
  layouts: [...LAYOUTS],
});

if (fileURLToPath(import.meta.url) === path.resolve(process.argv[1] || "")
    && process.argv.includes("--self-test")) {
  const sudoku = {
    version: 1,
    canvas: { width: 900, height: 900 },
    objects: [{
      type: "grid", x: 0, y: 0, width: 900, height: 900,
      rows: 9, columns: 9, majorEvery: 3, strokeWidth: "thin",
      majorStrokeWidth: "thick", color: "black",
      cells: [{ row: 0, column: 0, text: "5" }],
    }],
  };
  validateSceneToolCall(sudoku);
  const runs = compileScene(sudoku, { width: 800, height: 1_200 });
  if (runs.length < 2 || !runs.some((run) => run.style.width === "thick")) {
    throw new Error("scene self-test did not preserve grid line weights");
  }
  process.stdout.write("scene-self-test=ok\n");
}

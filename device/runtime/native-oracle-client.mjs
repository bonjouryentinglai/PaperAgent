#!/usr/bin/env node
// SPDX-License-Identifier: MIT

import net from "node:net";

const SOCKET = process.env.PAPER_AGENT_NATIVE_SOCKET || "/run/paper-agent-native-oracle.sock";
const args = process.argv.slice(2);
const health = args.length === 1 && args[0] === "--health";

if (!health && args.length !== 7) {
  console.error("usage: native-oracle-client.mjs ACTION PNG X Y WIDTH HEIGHT NEW_PAGE_REQUIRED | --health");
  process.exit(2);
}

const request = health
  ? { version: 1, type: "health" }
  : {
      version: 1,
      type: "write",
      action: args[0],
      png: args[1],
      x: Number(args[2]),
      y: Number(args[3]),
      width: Number(args[4]),
      height: Number(args[5]),
      newPageRequired: args[6] === "1",
    };

let accepted = false;
let finished = false;
let buffer = "";
const socket = net.createConnection(SOCKET);
const timeout = setTimeout(() => finish(accepted ? 1 : 75, "native oracle client timed out"), 305_000);

function finish(code, message) {
  if (finished) return;
  finished = true;
  clearTimeout(timeout);
  if (message) console.error(message);
  socket.destroy();
  process.exitCode = code;
}

socket.setEncoding("utf8");
socket.on("connect", () => socket.write(`${JSON.stringify(request)}\n`));
socket.on("data", (data) => {
  buffer += data;
  for (;;) {
    const newline = buffer.indexOf("\n");
    if (newline < 0) break;
    const line = buffer.slice(0, newline).trim();
    buffer = buffer.slice(newline + 1);
    if (!line) continue;
    let event;
    try { event = JSON.parse(line); }
    catch { finish(accepted ? 1 : 75, "native oracle returned malformed data"); return; }
    if (event.type === "ready") {
      console.log(`native_oracle=ready provider=${event.provider} model=${event.model} thinking=${event.thinking}`);
      finish(0);
    } else if (event.type === "accepted") {
      accepted = true;
      console.log(`native_oracle=accepted id=${event.id}`);
    } else if (event.type === "chunk") {
      console.log(`native_oracle_chunk=${event.index} kind=${event.kind || "ink"} elapsed_ms=${event.elapsedMs}`);
    } else if (event.type === "status") {
      console.log(`native_oracle_status=${event.stage} elapsed_ms=${event.elapsedMs}`);
    } else if (event.type === "done") {
      console.log(`native_oracle=done chunks=${event.chunks} elapsed_ms=${event.elapsedMs}`);
      finish(0);
    } else if (event.type === "error") {
      const unavailable = !accepted && ["warming", "busy"].includes(event.code);
      finish(unavailable ? 75 : 1, `native oracle: ${event.error || "request failed"}`);
    }
  }
});
socket.on("error", (error) => finish(accepted ? 1 : 75, `native oracle connection: ${error.message}`));
socket.on("end", () => {
  if (!finished) finish(accepted ? 1 : 75, "native oracle closed before completion");
});

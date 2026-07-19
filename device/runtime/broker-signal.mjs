#!/usr/bin/env node
// SPDX-License-Identifier: MIT

import fs from "node:fs";

const BROKER = process.env.PAPER_AGENT_XOVI_BROKER || "/run/xovi-mb";

function encodedSignal(signal, message) {
  if (!/^[a-z0-9_$-]+$/iu.test(signal)
      || typeof message !== "string" || message.length > 4096
      || /[\r\n:]/u.test(message)) {
    throw new Error("invalid XOVI broker signal");
  }
  return Buffer.from(`u${signal}:${message}\n`, "utf8");
}

function selfTest() {
  if (encodedSignal("paper-agent$status", "thinking").toString("utf8")
      !== "upaper-agent$status:thinking\n") {
    throw new Error("broker signal encoding failed");
  }
  for (const [signal, message] of [["bad:signal", "ok"], ["ok", "bad\nmessage"]]) {
    try {
      encodedSignal(signal, message);
      throw new Error("unsafe broker signal was accepted");
    } catch (error) {
      if (error.message === "unsafe broker signal was accepted") throw error;
    }
  }
  console.log("broker-signal-self-test=ok");
}

if (process.argv.length === 3 && process.argv[2] === "--self-test") {
  selfTest();
  process.exit(0);
}

if (process.argv.length !== 4) {
  console.error("usage: broker-signal.mjs SIGNAL MESSAGE | --self-test");
  process.exit(2);
}

let fd;
try {
  const payload = encodedSignal(process.argv[2], process.argv[3]);
  const info = fs.lstatSync(BROKER);
  if (!info.isFIFO() || info.isSymbolicLink()) {
    throw new Error("XOVI message broker is unavailable");
  }
  fd = fs.openSync(BROKER, fs.constants.O_WRONLY | fs.constants.O_NONBLOCK);
  const written = fs.writeSync(fd, payload);
  if (written !== payload.length) throw new Error("short XOVI broker write");
} catch (error) {
  console.error(`broker signal failed: ${String(error.message || error)}`);
  process.exitCode = error?.code === "ENXIO" || error?.code === "ENOENT" ? 75 : 1;
} finally {
  if (fd !== undefined) fs.closeSync(fd);
}

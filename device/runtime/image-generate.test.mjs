#!/usr/bin/env node
import assert from "node:assert/strict";
import {
  accountIdFor,
  buildRequestBody,
  decodeBase64Image,
  imagePayloadFromSse,
  validateCredential,
  validatePng,
} from "./image-generate.mjs";

const payload = Buffer.from(
  JSON.stringify({ "https://api.openai.com/auth": { chatgpt_account_id: "acct-test" } }),
).toString("base64url");
const token = `header.${payload}.signature`;
const credential = validateCredential({
  type: "oauth",
  access: token,
  refresh: "refresh-test",
  expires: Date.now() + 60_000,
});
assert.equal(accountIdFor(credential), "acct-test");

const image = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]).toString("base64");
const sse = [
  'data: {"type":"response.created"}',
  `data: ${JSON.stringify({
    type: "response.output_item.done",
    item: { type: "image_generation_call", result: image },
  })}`,
  "data: [DONE]",
].join("\n");
assert.equal(imagePayloadFromSse(sse), image);

const completed = `data: ${JSON.stringify({
  type: "response.completed",
  response: { output: [{ type: "image_generation_call", result: image }] },
})}`;
assert.equal(imagePayloadFromSse(completed), image);
const largeEncoded = Buffer.alloc(1024 * 1024, 0x5a).toString("base64");
assert.equal(decodeBase64Image(largeEncoded).length, 1024 * 1024);
assert.throws(() => decodeBase64Image("AA=A"), /malformed image data/);
assert.throws(() => decodeBase64Image("AA!A"), /malformed image data/);
assert.throws(
  () => imagePayloadFromSse('data: {"type":"response.failed","error":{"message":"private detail"}}'),
  /^Error: ChatGPT image generation failed$/,
);
assert.throws(() => validateCredential({ type: "api_key", key: "secret" }), /no usable/);

const body = buildRequestBody("A red apple on paper");
assert.equal(body.model, "gpt-5.6-sol");
assert.deepEqual(body.tool_choice, { type: "image_generation" });
assert.equal(body.tools[0].model, "gpt-image-2");
assert.equal(body.tools[0].output_format, "png");
assert.equal(body.tools[0].quality, "low");
assert.equal(body.stream, true);
assert.equal(body.store, false);

const onePixelPng = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
  "base64",
);
assert.equal(validatePng(onePixelPng), onePixelPng);
const corruptPng = Buffer.from(onePixelPng);
corruptPng[corruptPng.length - 6] ^= 0xff;
assert.throws(() => validatePng(corruptPng), /checksum|malformed|incomplete/);
assert.throws(() => validatePng(Buffer.from("not png")), /invalid PNG/);

process.stdout.write("image-generate tests passed\n");

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { parseSecretText, readSecretFile } from "../src/secret.js";

test("两行凭证可以读取", () => {
  assert.deepEqual(parseSecretText("admin\nsecret\n"), {
    secretState: "ready",
    username: "admin",
    password: "secret",
  });
});

test("行数不对时不可读", () => {
  assert.equal(parseSecretText("admin\n").secretState, "unreadable");
  assert.equal(parseSecretText("admin\n\n").secretState, "unreadable");
});

test("文件不存在时隐藏，读失败时不可读", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "sl-secret-"));
  assert.equal(readSecretFile(dir).secretState, "hidden");
  fs.writeFileSync(path.join(dir, "admin.secret"), "admin\nnext\n");
  assert.deepEqual(readSecretFile(dir), {
    secretState: "ready",
    username: "admin",
    password: "next",
  });
  fs.chmodSync(path.join(dir, "admin.secret"), 0);
  const blocked = readSecretFile(dir);
  assert.equal(blocked.secretState, "unreadable");
  fs.chmodSync(path.join(dir, "admin.secret"), 0o600);
});

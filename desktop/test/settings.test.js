import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { readSettings, readSettingsFile, writeSettingsFile } from "../src/settings.js";

test("默认端口是 8080", () => {
  assert.equal(readSettings(""), 8080);
  assert.equal(readSettings("{"), 8080);
  assert.equal(readSettings('{"port":70000}'), 8080);
  assert.equal(readSettings('{"port":"8080"}'), 8080);
});

test("端口写入后再读出仍是该值", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "sl-settings-"));
  const filePath = path.join(dir, "launcher.json");
  assert.equal(writeSettingsFile(filePath, 9090), true);
  assert.equal(readSettingsFile(filePath), 9090);
  assert.equal(writeSettingsFile(filePath, 0), false);
  assert.equal(readSettingsFile(filePath), 9090);
  assert.equal(readSettingsFile(path.join(dir, "missing.json")), 8080);
});

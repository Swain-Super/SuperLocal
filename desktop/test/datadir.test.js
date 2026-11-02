import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { moveDataDir, normalizeDataDir } from "../src/datadir.js";

test("空路径使用默认目录", () => {
  assert.equal(normalizeDataDir("  ", "/user/data"), path.resolve("/user/data"));
  assert.equal(normalizeDataDir("~/Library/sl", "/user/data"), path.join(os.homedir(), "Library/sl"));
});

test("当前目录不存在时只记下新路径", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sl-data-"));
  const next = path.join(root, "next");
  const moved = moveDataDir(path.join(root, "missing"), next);
  assert.equal(moved.ok, true);
  assert.equal(moved.dataDir, next);
  assert.equal(fs.existsSync(next), false);
});

test("把现有目录整份搬到新路径", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sl-data-"));
  const from = path.join(root, "data");
  const to = path.join(root, "moved");
  fs.mkdirSync(from);
  fs.writeFileSync(path.join(from, "superlocal.db"), "db");
  const moved = moveDataDir(from, to);
  assert.equal(moved.ok, true);
  assert.equal(fs.existsSync(from), false);
  assert.equal(fs.readFileSync(path.join(to, "superlocal.db"), "utf8"), "db");
});

test("目标目录有文件时不搬", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sl-data-"));
  const from = path.join(root, "data");
  const to = path.join(root, "taken");
  fs.mkdirSync(from);
  fs.writeFileSync(path.join(from, "superlocal.db"), "db");
  fs.mkdirSync(to);
  fs.writeFileSync(path.join(to, "keep.txt"), "keep");
  const moved = moveDataDir(from, to);
  assert.equal(moved.ok, false);
  assert.equal(moved.error, "目标目录不是空的");
  assert.equal(fs.readFileSync(path.join(from, "superlocal.db"), "utf8"), "db");
  assert.equal(fs.readFileSync(path.join(to, "keep.txt"), "utf8"), "keep");
});

test("不能搬到当前目录里面", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "sl-data-"));
  const from = path.join(root, "data");
  fs.mkdirSync(from);
  const moved = moveDataDir(from, path.join(from, "child"));
  assert.equal(moved.ok, false);
  assert.equal(moved.error, "目标路径和当前数据目录重叠");
  assert.equal(fs.existsSync(from), true);
});

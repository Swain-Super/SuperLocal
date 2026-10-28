import assert from "node:assert/strict";
import test from "node:test";
import { decidePid, exeFromCommand, parsePid } from "../src/pid.js";

const expected = "/Applications/SuperLocal.app/Contents/Resources/superlocal";

test("已退出或不是本次服务程序时丢弃", () => {
  assert.equal(decidePid({ alive: false, exePath: expected }, expected), "discard");
  assert.equal(decidePid({ alive: true, exePath: null }, expected), "discard");
  assert.equal(decidePid({ alive: true, exePath: "/usr/bin/other" }, expected), "discard");
});

test("可执行文件一致时采纳", () => {
  assert.equal(decidePid({ alive: true, exePath: expected }, expected), "adopt");
});

test("解析进程号和命令行", () => {
  assert.equal(parsePid("4242\n"), 4242);
  assert.equal(parsePid("0"), null);
  assert.equal(parsePid("nope"), null);
  assert.equal(exeFromCommand(`${expected} -addr :8080`), expected);
  assert.equal(exeFromCommand(`"${expected}" -addr :8080`), expected);
  assert.equal(exeFromCommand("   "), null);
});

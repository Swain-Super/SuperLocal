import assert from "node:assert/strict";
import test from "node:test";
import { beginQuit } from "../src/quit.js";

test("服务已停止时退出不再被窗口关闭取消", () => {
  assert.deepEqual(beginQuit(false, "stopped"), { quitting: true, hold: false });
  assert.deepEqual(beginQuit(false, "failed"), { quitting: true, hold: false });
  assert.deepEqual(beginQuit(false, undefined), { quitting: true, hold: false });
});

test("服务还在跑时先停服务再退出", () => {
  assert.deepEqual(beginQuit(false, "starting"), { quitting: true, hold: true });
  assert.deepEqual(beginQuit(false, "running"), { quitting: true, hold: true });
});

test("已经在退出时不再拦截", () => {
  assert.deepEqual(beginQuit(true, "running"), { quitting: true, hold: false });
});

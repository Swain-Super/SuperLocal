import assert from "node:assert/strict";
import test from "node:test";
import { explainStartupError, lastNonEmptyLine, requestStop, watchStartup } from "../src/supervisor.js";

function immediateDelay() {
  return () => Promise.resolve();
}

test("常见启动失败改成中文", () => {
  assert.equal(
    explainStartupError("listen tcp :8080: bind: address already in use\n"),
    "端口已被占用",
  );
  assert.equal(explainStartupError("listen tcp :80: bind: permission denied"), "没有权限监听这个端口");
  assert.equal(explainStartupError("服务在 10 秒内没有开始监听"), "服务在 10 秒内没有开始监听");
  assert.equal(explainStartupError(""), "服务启动失败");
});

test("标准错误取最后一行非空文本", () => {
  assert.equal(lastNonEmptyLine("first\n\nlisten tcp :8080: bind: address already in use\n"), "listen tcp :8080: bind: address already in use");
});

test("端口就绪时状态为运行中", async () => {
  const outcome = await watchStartup({
    exited: new Promise(() => {}),
    probe: async () => true,
    shouldStop: () => false,
    timeoutMs: 1000,
    delay: immediateDelay(),
    readStderr: () => "",
  });
  assert.deepEqual(outcome, { status: "running", error: "", kill: false });
});

test("进程退出时带上标准错误", async () => {
  const outcome = await watchStartup({
    exited: Promise.resolve("listen tcp :9090: bind: address already in use\n"),
    probe: async () => false,
    shouldStop: () => false,
    timeoutMs: 1000,
    delay: immediateDelay(),
    readStderr: () => "",
  });
  assert.equal(outcome.status, "failed");
  assert.equal(outcome.error, "listen tcp :9090: bind: address already in use");
  assert.equal(outcome.kill, false);
});

test("超时要结束进程", async () => {
  const outcome = await watchStartup({
    exited: new Promise(() => {}),
    probe: async () => false,
    shouldStop: () => false,
    timeoutMs: 400,
    delay: immediateDelay(),
    readStderr: () => "",
  });
  assert.deepEqual(outcome, {
    status: "failed",
    error: "服务在 10 秒内没有开始监听",
    kill: true,
  });
});

test("用户停止时不算启动失败", async () => {
  const outcome = await watchStartup({
    exited: new Promise(() => {}),
    probe: async () => false,
    shouldStop: () => true,
    timeoutMs: 1000,
    delay: immediateDelay(),
    readStderr: () => "",
  });
  assert.deepEqual(outcome, { status: "stopped", error: "", kill: true });
});

test("停止会先请求结束，超时后强制结束", async () => {
  const signals = [];
  let alive = true;
  const outcome = await requestStop({
    signal: (name) => {
      signals.push(name);
      if (name === "kill") {
        alive = false;
      }
    },
    isAlive: () => alive,
    delay: immediateDelay(),
    waitMs: 200,
  });
  assert.deepEqual(signals, ["term", "kill"]);
  assert.deepEqual(outcome, { ok: true, error: "" });
});

test("强制结束后仍在则停止失败", async () => {
  const outcome = await requestStop({
    signal: () => {},
    isAlive: () => true,
    delay: immediateDelay(),
    waitMs: 100,
  });
  assert.deepEqual(outcome, { ok: false, error: "停止服务失败" });
});

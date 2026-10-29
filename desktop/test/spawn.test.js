import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { spawn } from "node:child_process";
import { urlsFromBanner } from "../src/banner.js";
import { readSecretFile } from "../src/secret.js";
import { requestStop, watchStartup } from "../src/supervisor.js";

const binary = path.join(import.meta.dirname, "..", "binaries", "superlocal");

test("服务程序能按端口启动并写出管理员密码", async (t) => {
  if (!fs.existsSync(binary)) {
    t.skip("没有编译好的服务程序");
    return;
  }
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), "sl-launch-"));
  const port = 18083;
  let stdoutText = "";
  let stderrText = "";
  const proc = spawn(binary, ["-addr", `:${port}`, "-data", dataDir, "-max-upload", "200MB"], {
    stdio: ["ignore", "pipe", "pipe"],
  });
  proc.stdout.on("data", (buf) => {
    stdoutText += buf.toString();
  });
  proc.stderr.on("data", (buf) => {
    stderrText += buf.toString();
  });
  const exited = new Promise((resolve) => {
    proc.once("exit", () => resolve(stderrText));
    proc.once("error", (err) => resolve(err.message));
  });
  const outcome = await watchStartup({
    exited,
    probe: async () => {
      try {
        await fetch(`http://127.0.0.1:${port}/`, { signal: AbortSignal.timeout(1000) });
        return true;
      } catch {
        return false;
      }
    },
    shouldStop: () => false,
    timeoutMs: 10000,
    delay: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
    readStderr: () => stderrText,
  });
  assert.equal(outcome.status, "running");
  const urls = urlsFromBanner(stdoutText, port);
  assert.equal(urls.chatUrls.some((url) => url.includes("127.0.0.1")), false);
  assert.equal(urls.adminUrls.some((url) => url.includes("127.0.0.1")), false);
  const secret = readSecretFile(dataDir);
  assert.equal(secret.secretState, "ready");
  assert.equal(secret.username, "admin");
  assert.match(secret.password, /^[A-Za-z0-9]{16}$/);

  let alive = true;
  proc.once("exit", () => {
    alive = false;
  });
  const stopped = await requestStop({
    signal: (kind) => {
      try {
        process.kill(proc.pid, kind === "kill" ? "SIGKILL" : "SIGTERM");
      } catch {
        alive = false;
      }
    },
    isAlive: () => alive,
    delay: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
  });
  assert.equal(stopped.ok, true);
  fs.rmSync(dataDir, { recursive: true, force: true });
});

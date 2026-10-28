import assert from "node:assert/strict";
import test from "node:test";
import path from "node:path";
import { launcherPaths, serviceBinaryPath } from "../src/paths.js";

test("开发和打包使用不同的服务程序路径", () => {
  assert.equal(
    serviceBinaryPath({
      isPackaged: false,
      resourcesPath: "/resources",
      appRoot: "/app",
      platform: "darwin",
    }),
    path.join("/app", "binaries", "superlocal"),
  );
  assert.equal(
    serviceBinaryPath({
      isPackaged: true,
      resourcesPath: "/resources",
      appRoot: "/app",
      platform: "win32",
    }),
    path.join("/resources", "superlocal.exe"),
  );
});

test("用户数据目录里的文件位置固定", () => {
  assert.deepEqual(launcherPaths("/user"), {
    settings: path.join("/user", "launcher.json"),
    pid: path.join("/user", "superlocal.pid"),
    dataDir: path.join("/user", "data"),
  });
});

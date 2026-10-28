import { app, BrowserWindow, Menu, Tray, ipcMain, nativeImage, shell } from "electron";
import { spawn, execFile } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { localOnlyUrls, urlsFromBanner } from "./banner.js";
import { launcherPaths, serviceBinaryPath } from "./paths.js";
import { decidePid, exeFromCommand, parsePid } from "./pid.js";
import { parsePort } from "./port.js";
import { readSecretFile } from "./secret.js";
import { readSettingsFile, writeSettingsFile } from "./settings.js";
import { lastNonEmptyLine, requestStop, watchStartup } from "./supervisor.js";

let win = null;
let tray = null;
let childPid = null;
let stopRequested = false;
let quitting = false;
let startPromise = null;
let stdoutText = "";
let stderrText = "";
let state = null;

function userPaths() {
  return launcherPaths(app.getPath("userData"));
}

function binaryPath() {
  return serviceBinaryPath({
    isPackaged: app.isPackaged,
    resourcesPath: process.resourcesPath,
    appRoot: app.getAppPath(),
  });
}

function publish() {
  if (win && !win.isDestroyed()) {
    win.webContents.send("launcher:state", state);
  }
  rebuildTray();
}

function refreshSecret() {
  const secret = readSecretFile(userPaths().dataDir);
  state.secretState = secret.secretState;
  state.username = secret.username;
  state.password = secret.password;
}

function showWindow() {
  if (!win || win.isDestroyed()) {
    return;
  }
  if (win.isMinimized()) {
    win.restore();
  }
  win.show();
  win.focus();
  refreshSecret();
  publish();
}

function isAlive(pid) {
  if (!pid) {
    return false;
  }
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function killPid(pid, kind) {
  if (!pid) {
    return;
  }
  try {
    process.kill(pid, kind === "kill" ? "SIGKILL" : "SIGTERM");
  } catch {
    // 进程已经不在。
  }
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function readExe(pid) {
  return new Promise((resolve) => {
    if (!/^[1-9][0-9]*$/.test(String(pid))) {
      resolve(null);
      return;
    }
    if (process.platform === "win32") {
      execFile(
        "powershell.exe",
        [
          "-NoProfile",
          "-Command",
          `(Get-CimInstance Win32_Process -Filter "ProcessId=${pid}").ExecutablePath`,
        ],
        { timeout: 3000 },
        (err, out) => resolve(err ? null : String(out).trim() || null),
      );
      return;
    }
    execFile("ps", ["-p", String(pid), "-o", "args="], { timeout: 3000 }, (err, out) => {
      resolve(err ? null : exeFromCommand(out));
    });
  });
}

function writePid(pid) {
  fs.mkdirSync(app.getPath("userData"), { recursive: true });
  fs.writeFileSync(userPaths().pid, `${pid}\n`);
}

function clearPid() {
  fs.rmSync(userPaths().pid, { force: true });
}

async function adoptLeftover() {
  let text = "";
  try {
    text = fs.readFileSync(userPaths().pid, "utf8");
  } catch {
    return;
  }
  const pid = parsePid(text);
  if (!pid || !isAlive(pid)) {
    clearPid();
    return;
  }
  const exe = await readExe(pid);
  if (decidePid({ alive: true, exePath: exe }, binaryPath()) !== "adopt") {
    clearPid();
    return;
  }
  childPid = pid;
  state.status = "running";
  state.error = "";
  const urls = localOnlyUrls(state.port);
  state.chatUrls = urls.chatUrls;
  state.adminUrls = urls.adminUrls;
}

async function finishStop(pid) {
  const outcome = await requestStop({
    signal: (kind) => killPid(pid, kind),
    isAlive: () => isAlive(pid),
    delay,
  });
  if (outcome.ok || !isAlive(pid)) {
    if (childPid === pid) {
      childPid = null;
    }
    clearPid();
    state.status = "stopped";
    state.error = "";
    return;
  }
  childPid = pid;
  state.status = "running";
  state.error = "停止服务失败";
}

async function stopService() {
  if (state.status === "starting") {
    stopRequested = true;
    if (startPromise) {
      await startPromise;
    }
    return state;
  }
  if (state.status !== "running") {
    state.error = "";
    publish();
    return state;
  }
  await finishStop(childPid);
  stopRequested = false;
  publish();
  return state;
}

async function probeHttp(port) {
  try {
    await fetch(`http://127.0.0.1:${port}/`, { signal: AbortSignal.timeout(1000) });
    return true;
  } catch {
    return false;
  }
}

async function runStart(port) {
  stopRequested = false;
  stdoutText = "";
  stderrText = "";
  state.status = "starting";
  state.error = "";
  state.port = port;
  writeSettingsFile(userPaths().settings, port);
  publish();

  const binary = binaryPath();
  const proc = spawn(
    binary,
    ["-addr", `:${port}`, "-data", userPaths().dataDir, "-max-upload", "200MB"],
    { stdio: ["ignore", "pipe", "pipe"], windowsHide: true },
  );
  childPid = proc.pid;
  writePid(proc.pid);
  proc.stdout.on("data", (buf) => {
    stdoutText += buf.toString();
  });
  proc.stderr.on("data", (buf) => {
    stderrText += buf.toString();
  });

  let settled = false;
  const exited = new Promise((resolve) => {
    const finish = (text) => {
      if (settled) {
        return;
      }
      settled = true;
      resolve(text);
    };
    proc.once("exit", () => finish(stderrText));
    proc.once("error", (err) => finish(err.message));
  });

  const outcome = await watchStartup({
    exited,
    probe: () => probeHttp(port),
    shouldStop: () => stopRequested,
    timeoutMs: 10000,
    delay,
    readStderr: () => stderrText,
  });

  if (outcome.kill) {
    await finishStop(proc.pid);
  }

  if (stopRequested || outcome.status === "stopped") {
    if (isAlive(proc.pid)) {
      childPid = proc.pid;
      state.status = "running";
      state.error = "停止服务失败";
    } else {
      childPid = null;
      clearPid();
      state.status = "stopped";
      state.error = "";
    }
    refreshSecret();
    publish();
    return;
  }

  if (outcome.status === "running") {
    const urls = urlsFromBanner(stdoutText, port);
    state.chatUrls = urls.chatUrls;
    state.adminUrls = urls.adminUrls;
    state.status = "running";
    state.error = "";
    childPid = proc.pid;
    refreshSecret();
    publish();
    proc.once("exit", () => {
      if (childPid !== proc.pid || state.status !== "running") {
        return;
      }
      childPid = null;
      clearPid();
      state.status = "stopped";
      state.error = lastNonEmptyLine(stderrText);
      refreshSecret();
      publish();
    });
    return;
  }

  if (isAlive(proc.pid)) {
    childPid = proc.pid;
    state.status = "running";
    state.error = outcome.error;
  } else {
    childPid = null;
    clearPid();
    state.status = "failed";
    state.error = outcome.error;
    if (stdoutText.includes("http://")) {
      const urls = urlsFromBanner(stdoutText, port);
      state.chatUrls = urls.chatUrls;
      state.adminUrls = urls.adminUrls;
    }
  }
  refreshSecret();
  publish();
}

async function startService(rawPort) {
  if (state.status === "starting" || state.status === "running") {
    return state;
  }
  const parsed = parsePort(rawPort ?? state.port);
  if (!parsed.ok) {
    state.error = "请输入 1 到 65535 的端口";
    publish();
    return state;
  }
  if (!fs.existsSync(binaryPath())) {
    state.status = "failed";
    state.error = "找不到服务程序";
    publish();
    return state;
  }
  startPromise = runStart(parsed.port);
  try {
    await startPromise;
  } finally {
    startPromise = null;
    stopRequested = false;
  }
  return state;
}

function updatePort(rawPort) {
  if (state.status === "starting" || state.status === "running") {
    return state;
  }
  const parsed = parsePort(rawPort);
  if (!parsed.ok) {
    return state;
  }
  state.port = parsed.port;
  state.error = "";
  writeSettingsFile(userPaths().settings, parsed.port);
  publish();
  return state;
}

function openLocal(kind) {
  if (state.status !== "running") {
    return;
  }
  const url =
    kind === "admin"
      ? `http://127.0.0.1:${state.port}/admin`
      : `http://127.0.0.1:${state.port}/`;
  shell.openExternal(url);
}

function rebuildTray() {
  if (!tray || !state) {
    return;
  }
  const busy = state.status === "starting" || state.status === "running";
  tray.setContextMenu(
    Menu.buildFromTemplate([
      { label: "显示窗口", click: () => showWindow() },
      {
        label: busy ? "停止服务" : "启动服务",
        click: () => {
          if (busy) {
            stopService();
          } else {
            startService(state.port);
          }
        },
      },
      {
        label: "打开聊天",
        enabled: state.status === "running",
        click: () => openLocal("chat"),
      },
      {
        label: "打开后台",
        enabled: state.status === "running",
        click: () => openLocal("admin"),
      },
      { type: "separator" },
      {
        label: "退出",
        click: () => {
          quitting = true;
          stopService().finally(() => app.quit());
        },
      },
    ]),
  );
}

function createWindow() {
  win = new BrowserWindow({
    width: 420,
    height: 640,
    minWidth: 380,
    minHeight: 520,
    title: "SuperLocal",
    backgroundColor: "#F3F5F7",
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(app.getAppPath(), "src/preload.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  win.loadFile(path.join(app.getAppPath(), "dist/index.html"));
  win.on("close", (event) => {
    if (!quitting) {
      event.preventDefault();
      win.hide();
    }
  });
  win.on("show", () => {
    refreshSecret();
    publish();
  });
}

function createTray() {
  const image = nativeImage.createFromPath(path.join(app.getAppPath(), "assets/tray.png"));
  tray = new Tray(image.isEmpty() ? nativeImage.createEmpty() : image);
  tray.setToolTip("SuperLocal");
  tray.on("click", () => showWindow());
  rebuildTray();
}

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  app.quit();
} else {
  app.setName("SuperLocal");
  app.on("second-instance", () => showWindow());
  app.on("window-all-closed", () => {});
  app.on("activate", () => showWindow());
  app.on("before-quit", (event) => {
    if (quitting) {
      return;
    }
    if (state && (state.status === "starting" || state.status === "running")) {
      event.preventDefault();
      quitting = true;
      stopService().finally(() => app.quit());
    }
  });

  app.whenReady().then(async () => {
    const port = readSettingsFile(userPaths().settings);
    state = {
      status: "stopped",
      port,
      error: "",
      chatUrls: [],
      adminUrls: [],
      username: "",
      password: "",
      secretState: "hidden",
    };
    refreshSecret();
    await adoptLeftover();
    ipcMain.handle("launcher:getState", () => state);
    ipcMain.handle("launcher:start", (_event, portValue) => startService(portValue));
    ipcMain.handle("launcher:stop", () => stopService());
    ipcMain.handle("launcher:setPort", (_event, portValue) => updatePort(portValue));
    ipcMain.handle("launcher:openChat", () => {
      openLocal("chat");
      return state;
    });
    ipcMain.handle("launcher:openAdmin", () => {
      openLocal("admin");
      return state;
    });
    createWindow();
    createTray();
    publish();
  });
}

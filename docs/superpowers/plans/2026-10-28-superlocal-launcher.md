# SuperLocal Launcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 做一个 Electron 服务管理壳，在没有 Go 和 Node 的电脑上打开管理页、按所选端口启动现有 `superlocal`，并缩到托盘。

**Architecture:** 纯函数模块负责端口、横幅、设置、进程号和启动停止判断。Electron 主进程拉起 Go 子进程并托管本地 Vue 管理页。聊天和后台仍用系统浏览器打开现有 HTTP 服务。

**Tech Stack:** Electron、electron-builder、Vue 3、Vite、Node 自带 `node:test`。服务程序仍是 `CGO_ENABLED=0` 的 Go 二进制。

## Global Constraints

- 双击只打开管理窗口，服务保持停止，直到用户点「启动」。
- 端口可改，默认 `8080`，范围是 `1` 到 `65535` 的整数。启动参数是 `-addr :{port}`、`-data {userData}/data`、`-max-upload 200MB`。
- 关掉窗口只隐藏。托盘「退出」和 macOS 退出在启动中或运行中时先停服务。
- 本机健康检查是 `http://127.0.0.1:{port}/`，10 秒内任意 HTTP 状态码算成功。
- 账号密码只读 `{userData}/data/admin.secret`。地址优先用服务标准输出里的 `http://` 行。
- 不修改 Go 服务的 HTTP、WebSocket 和 48 小时清理，不做 WebRTC。
- 管理页不用 Element Plus，不新增组件库。颜色与规格一致。
- macOS 在 Mac 上打 `.dmg`，Windows 在 Windows 上打安装程序。对方电脑不安装 Go 或 Node。

## File Structure

| 路径 | 职责 |
| --- | --- |
| `desktop/src/port.js` | 校验端口 |
| `desktop/src/banner.js` | 解析标准输出中的地址 |
| `desktop/src/settings.js` | 读写 `launcher.json` |
| `desktop/src/secret.js` | 解析 `admin.secret` |
| `desktop/src/pid.js` | 判断残留进程，解析进程号和命令行 |
| `desktop/src/supervisor.js` | 轮询就绪、判定启动结果、请求停止 |
| `desktop/src/main.js` | 窗口、托盘、子进程、IPC |
| `desktop/src/preload.cjs` | 向页面暴露 IPC |
| `desktop/renderer/` | Vue 管理页 |
| `desktop/electron-builder.yml` | dmg 与 Windows 安装程序 |

---

### Task 1: 端口、地址、设置、凭证和进程号

**Files:**
- Create: `desktop/src/port.js`
- Create: `desktop/src/banner.js`
- Create: `desktop/src/settings.js`
- Create: `desktop/src/secret.js`
- Create: `desktop/src/pid.js`
- Test: `desktop/test/port.test.js`
- Test: `desktop/test/banner.test.js`
- Test: `desktop/test/settings.test.js`
- Test: `desktop/test/secret.test.js`
- Test: `desktop/test/pid.test.js`

**Interfaces:**
- Consumes: 无
- Produces: `parsePort(value) -> { ok: true, port } | { ok: false }`
- Produces: `parseBanner(text) -> { chatUrls, adminUrls }`
- Produces: `urlsFromBanner(text, port)`、`localOnlyUrls(port)`
- Produces: `readSettings(jsonText) -> number`、`writeSettingsFile(filePath, port) -> boolean`
- Produces: `parseSecretText(text) -> { secretState, username, password }`
- Produces: `decidePid(probe, expectedExe) -> 'adopt' | 'discard'`
- Produces: `parsePid(text) -> number | null`、`exeFromCommand(commandLine) -> string | null`

- [ ] **Step 1: 写失败测试并实现上述函数**

`parsePort` 接受整数和整数字符串。`readSettings` 只接受 JSON 数字端口，缺失、损坏或超范围时返回 `8080`。`parseSecretText` 要求去掉末尾换行后恰好两行非空。`decidePid` 仅在 `probe.alive` 且 `probe.exePath === expectedExe` 时返回 `adopt`。

- [ ] **Step 2: 运行 `node --test`**

Run: `node --test desktop/test/port.test.js desktop/test/banner.test.js desktop/test/settings.test.js desktop/test/secret.test.js desktop/test/pid.test.js`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add desktop/src/port.js desktop/src/banner.js desktop/src/settings.js desktop/src/secret.js desktop/src/pid.js desktop/test
git commit -m "feat(launcher): 添加端口、地址和进程判断"
```

### Task 2: 启动与停止判断

**Files:**
- Create: `desktop/src/supervisor.js`
- Test: `desktop/test/supervisor.test.js`

**Interfaces:**
- Consumes: 无 Electron
- Produces: `pollReady({ probe, shouldStop, timeoutMs, delay }) -> 'ready' | 'stopped' | 'timeout'`
- Produces: `watchStartup({ exited, probe, shouldStop, timeoutMs, delay, readStderr })`
- Produces: `requestStop({ signal, isAlive, delay, waitMs }) -> { ok, error }`
- Produces: `lastNonEmptyLine(text) -> string`

`watchStartup` 在用户停止时返回 `status: 'stopped'`。进程退出时 `error` 是标准错误最后一行非空文本。超时返回 `服务在 10 秒内没有开始监听` 且 `kill: true`。探测成功返回 `status: 'running'`。`requestStop` 先发 `term`，`waitMs` 后仍存活再发 `kill`；仍存活则 `{ ok: false, error: '停止服务失败' }`。

- [ ] **Step 1: 写测试并实现**
- [ ] **Step 2: 运行 `node --test desktop/test/supervisor.test.js`，预期 PASS**
- [ ] **Step 3: Commit `feat(launcher): 添加启动和停止判断`**

### Task 3: Electron 主进程和管理页

**Files:**
- Create: `desktop/package.json`
- Create: `desktop/vite.config.js`
- Create: `desktop/src/main.js`
- Create: `desktop/src/preload.cjs`
- Create: `desktop/src/paths.js`
- Create: `desktop/renderer/index.html`
- Create: `desktop/renderer/main.js`
- Create: `desktop/renderer/App.vue`
- Create: `desktop/renderer/style.css`
- Create: `desktop/assets/tray.png`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: Task 1 和 Task 2 的导出
- Produces: IPC `launcher:getState`、`launcher:start`、`launcher:stop`、`launcher:setPort`、`launcher:openChat`、`launcher:openAdmin`，以及推送 `launcher:state`

行为：

- `requestSingleInstanceLock`。第二实例只显示窗口。
- 窗口默认 420×640，最小 380×520。关闭改为隐藏。`window-all-closed` 不退出。
- 托盘菜单：显示窗口、启动或停止、打开聊天、打开后台、退出。
- 开发时二进制在 `desktop/binaries/`，打包后在 `process.resourcesPath`。
- 找不到二进制时不创建子进程，`error` 为「找不到服务程序」。
- 合法端口在已停止或启动失败时写入 `launcher.json`。启动中或运行中拒绝修改。
- 打开窗口和进入运行中时重读 `admin.secret`。
- 采纳残留进程后只填本机两个地址。
- 「打开聊天」「打开后台」仅在运行中调用 `shell.openExternal`，地址是 `http://127.0.0.1:{port}/` 和 `http://127.0.0.1:{port}/admin`。

- [ ] **Step 1: 安装 Vue、Vite、Electron，构建页面**
- [ ] **Step 2: 用 Node 24 执行 `npm test` 与 `npm run build:renderer`**
- [ ] **Step 3: Commit `feat(launcher): 添加服务管理窗口`**

### Task 4: 打包说明

**Files:**
- Create: `desktop/electron-builder.yml`
- Modify: `README.md`

- [ ] **Step 1: macOS 目标为 dmg 且不签名。Windows 目标为 nsis，只在 Windows 上执行。extraResources 放入 `desktop/binaries` 里的服务程序。**
- [ ] **Step 2: README 写明 Node 24、Go 编译命令，以及对方机器不需要安装 Go 或 Node。**
- [ ] **Step 3: `CGO_ENABLED=0 go test ./...` 仍通过。**
- [ ] **Step 4: Commit `docs: 说明服务管理壳的打包方式`**

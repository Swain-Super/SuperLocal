# 单聊屏幕共享 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 单聊里一方共享自己的屏幕，对方只能看；这一路和语音、视频可以同时进行。

**Architecture:** `internal/screen` 单独记录屏幕共享，不读写 `internal/call`。WebSocket 增加 `screen.*` 信令。浏览器在对方接受之后才调用 `getDisplayMedia`，画面走现有 WebRTC 和 TURN。

**Tech Stack:** Go 1.22、现有 `github.com/coder/websocket`、Vue 3、浏览器 `RTCPeerConnection` 和 `getDisplayMedia`。不新增前端依赖。

## Global Constraints

- 只有 `direct` 会话显示「共享屏幕」。远程控制、剪贴板、系统声音、麦克风、群共享、共享记录、涂鸦不做。
- 屏幕共享和通话各用各的会话。一个用户同时只在一路屏幕共享中。正在通话不是 `busy`，正在共享也不是通话的 `busy`。
- `-tls=false` 时不创建 screen 会话。`GET /api/features` 仍只返回 `{"calls": true}`。`calls` 为假时不显示按钮，也不处理 `screen.*`。
- `screenId` 由服务生成。30 秒超时只覆盖尚未接受的阶段，原因是 `timeout`。接受后不因选择屏幕而超时。
- 发起方确认有 `getDisplayMedia` 后才发送 `screen.invite`。对方接受后才抓屏幕。只要画面，`audio: false`。优先 `cursor: "always"`，约束不被接受时去掉再请求一次。
- 接受前的 offer 不转发。只有发起方发 offer，只有观看方发 answer。
- 某个 WebSocket 关闭时，通话和共享各自结束，规则与现有通话相同。
- 观看画面使用 `object-fit: contain`。发起方不显示自己屏幕的大画面。
- 两路同时进行时，屏幕占满，通话的静音、摄像头和挂断收在右下角。两路邀请同时出现时，后到的一层盖在上面。
- 颜色沿用 `#0b6bcb`、`#1f2329`、`#e7f0fa`、`#e11d48`、`#646a73`。聊天页不使用 Element Plus。
- `CGO_ENABLED=0`。不新增设备字段。桌面程序不改。

## File Structure

| 路径 | 职责 |
| --- | --- |
| `internal/screen/session.go` | 共享状态、忙线、超时、信令转发 |
| `internal/screen/session_test.go` | 会话规则 |
| `internal/httpapi/screen.go` | 把 `screen.*` 交给 `screen.Session` |
| `internal/httpapi/screen_test.go` | 信令与通话互不占用 |
| `internal/httpapi/server.go` | 注入会话并每秒清扫 |
| `internal/httpapi/ws.go` | 接收事件，断开时结束共享 |
| `cmd/superlocal/main.go` | 在 HTTPS 开启时创建 screen 会话 |
| `web/src/screenState.ts` | 与浏览器无关的阶段 |
| `web/src/screenState.test.ts` | 阶段转移 |
| `web/src/screen.ts` | `getDisplayMedia` 和 `RTCPeerConnection` |
| `web/src/stores/chat.ts` | 把 `screen.*` 交给页面 |
| `web/src/views/Chat.vue` | 按钮和浮层 |

### Task 1: 屏幕共享会话

**Files:**
- Create: `internal/screen/session.go`
- Test: `internal/screen/session_test.go`

**Interfaces:**
- Produces: `screen.New(now, id, turn) *Session`，`Invite`，`Accept`，`Reject`，`Hangup`，`Offer`，`Answer`，`Candidate`，`Disconnect`，`Sweep`。事件字段 `screenId`。

- [ ] **Step 1: Write the failing test**

覆盖：三人列表和发起者不在成员里是 `forbidden`；离线是 `offline`；已有共享是 `busy`；30 秒未接受是 `timeout`，之后可以再发起；接受后清扫不结束；接受前 offer 为空；观看方的 offer 和发起方的 answer 不转发；拒绝、挂断、断开连接。

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/screen/`
Expected: FAIL，包不存在。

- [ ] **Step 3: Write minimal implementation**

`Invite` 不接收 `kind`。`Accept` 向发起方发 `screen.accept`，向观看方发 `screen.end`。`Sweep` 在 `accepted` 为真时跳过。`Offer` 要求已接受且发送者是发起方。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/screen/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/screen/session.go internal/screen/session_test.go
git commit -m "$(cat <<'EOF'
feat(screen): 维护单聊屏幕共享状态

EOF
)"
```

### Task 2: 转发信令

**Files:**
- Create: `internal/httpapi/screen.go`
- Create: `internal/httpapi/screen_test.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/ws.go`
- Modify: `cmd/superlocal/main.go`

**Interfaces:**
- Consumes: `screen.Session` 的方法和 `screen.Delivery`
- Produces: `Deps.Screens *screen.Session`。WebSocket 接受 `screen.invite`、`screen.accept`、`screen.reject`、`screen.hangup`、`screen.offer`、`screen.answer`、`screen.candidate`。

- [ ] **Step 1: Write the failing HTTP test**

同一对用户先建立通话，再发起屏幕共享，观看方仍能收到 `screen.invite`。把会话改成 `group` 时原因是 `forbidden`。已有共享时再次发起是 `busy`。接受前的 `screen.offer` 不转发给对方。关闭观看方连接后，发起方收到 `screen.hangup`。

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/httpapi/ -run TestScreenSignaling`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`server` 增加 `screens`。清扫和通话放在同一个每秒循环里，各自调用。`wsIncoming` 增加 `screenId`。连接关闭时分别 `Disconnect`。`main` 只在 `cfg.TLS` 时创建 screen 会话，TURN 凭据函数和通话分开铸造。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/httpapi/ -run 'TestScreenSignaling|TestCallSignaling'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/screen.go internal/httpapi/screen_test.go internal/httpapi/server.go internal/httpapi/ws.go cmd/superlocal/main.go
git commit -m "$(cat <<'EOF'
feat(screen): 转发屏幕共享信令

EOF
)"
```

### Task 3: 页面阶段

**Files:**
- Create: `web/src/screenState.ts`
- Test: `web/src/screenState.test.ts`

**Interfaces:**
- Produces: `ScreenPhase`、`ScreenView`、`idleScreen`、`outgoingScreen`、`reduceScreen`、`screenStatusText`

阶段：`idle`、`outgoing`、`incoming`、`choosing`、`connecting`、`active`、`failed`。`screen.accept` 把发起方从 `outgoing` 变成 `choosing`，不进入 `connecting`。观看方在 `connecting` 时忽略 `screen.end`。`screen.hangup` 在观看方得到原因 `sharer-end`，在发起方得到 `viewer-end`。

- [ ] **Step 1: Write the failing test**
- [ ] **Step 2: Run** `PATH="/usr/local/opt/node@24/bin:$PATH" node --experimental-strip-types --test web/src/screenState.test.ts`，Expected: FAIL
- [ ] **Step 3: Implement**
- [ ] **Step 4: Re-run**，Expected: PASS
- [ ] **Step 5: Commit** `feat(web): 记录屏幕共享界面状态`

### Task 4: 浏览器连接

**Files:**
- Create: `web/src/screen.ts`
- Modify: `web/src/stores/chat.ts`

**Interfaces:**
- Consumes: `reduceScreen`、`outgoingScreen`
- Produces: `screen`、`remoteScreen`、`canShareScreen`、`startScreen`、`acceptScreen`、`rejectScreen`、`hangupScreen`、`dismissScreen`、`onScreenSignal`、`bindScreenSender`

`startScreen` 在没有 `getDisplayMedia` 时返回 `false`，不发送邀请。`onScreenSignal` 在阶段变成 `choosing` 后才抓屏并发送 offer。观看方接受时不抓屏。`callsEnabled` 为假时 `chat.ts` 忽略 `screen.*`。

- [ ] **Step 1: Implement `screen.ts` and wire `chat.ts`**
- [ ] **Step 2: Run** `PATH="/usr/local/opt/node@24/bin:$PATH" node --experimental-strip-types --test web/src/screenState.test.ts`，Expected: PASS
- [ ] **Step 3: Commit** `feat(web): 建立屏幕共享连接`

### Task 5: 聊天页按钮和浮层

**Files:**
- Modify: `web/src/views/Chat.vue`

单聊标题栏在「视频」后加「共享屏幕」。没有 `getDisplayMedia` 时按钮旁显示「当前设备无法共享屏幕」。浮层复用通话按钮样式。观看方的画面类名 `screen-remote`，`object-fit: contain`。`call-layer-dock` 把进行中的通话收成右下角。邀请层用后到的那一路加 `call-layer-front` 或 `screen-layer-front`。

- [ ] **Step 1: Add the button, layer, and CSS**
- [ ] **Step 2: Run** `cd web && PATH="/usr/local/opt/node@24/bin:$PATH" npm run build`，Expected: PASS
- [ ] **Step 3: Commit** `feat(web): 单聊可以共享屏幕`

## Self-review

- 会话范围、信令顺序、界面叠层、错误文案和测试都落在 Task 1 到 Task 5。
- 桌面程序没有任务。`/api/features` 不增加字段。
- `screenId`、`reduceScreen`、`startScreen` 在前后任务里使用同一个名字。

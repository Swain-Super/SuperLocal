# 一对一 WebRTC 通话设计

单聊增加语音通话和视频通话。音视频在两台浏览器之间用 WebRTC 直接传输。现有 WebSocket 只转发通话信令。文字、图片、文件和在线状态仍由服务转发，不改成直连。

本设计替换《SuperLocal 设计》里「程序内置 HTTPS 不在这一版」和「程序只提供 HTTP」这两句。群通话、屏幕共享、通话记录不在这一版。

## 运行形态

服务默认在原来的 `-addr` 上提供 HTTPS，不再在该端口提供 HTTP。桌面程序打印的聊天地址、管理地址和健康检查都使用 `https://`。

第一次启动时，在数据目录生成自签证书：

- `data/tls/cert.pem`
- `data/tls/key.pem`，文件权限 `0600`

证书使用 ECDSA P-256，有效期两年。使用者可选名称包含 `localhost`、`127.0.0.1` 和启动时发现的局域网 IPv4。证书缺失、过期，或局域网地址集合和证书里的地址不一致时，重新生成。重新生成后，浏览器需要再次确认证书警告。

对方用 `https://<局域网地址>:<端口>/` 打开。第一次会出现不受信任的证书警告，继续访问之后，这个页面才成为安全上下文，浏览器才允许使用麦克风和摄像头。

`-tls=false` 时服务仍只提供 HTTP，供已经在反向代理上终止 TLS 的部署使用。此时聊天页不显示通话按钮，进程不监听 TURN。桌面程序在默认配置下不传这个参数。

TURN 使用和 HTTPS 相同的端口号，协议是 UDP。例如 HTTPS 在 `:8080` 时，TURN 在 UDP `8080`。浏览器优先走局域网直连，直连失败时经这台 SuperLocal 主机中转音视频。

## 技术划分

| 部分 | 职责 |
| --- | --- |
| `internal/tlscert` | 生成、保存和加载自签证书 |
| `internal/call` | 通话状态、成员校验、信令转发、忙线和超时 |
| `cmd/superlocal` | 按配置启动 HTTPS 和 TURN |
| `web/src/call.ts` | 浏览器侧 `RTCPeerConnection` 和设备权限 |
| `web/src/views/Chat.vue` | 单聊按钮、来电层和通话浮层 |
| `desktop/src` | 健康检查和地址展示改为 `https://` |

不新增前端依赖。通话使用浏览器内置的 `RTCPeerConnection` 和 `getUserMedia`。聊天页仍不使用 Element Plus。服务端 TURN 使用 `github.com/pion/turn/v4`。

通话状态只留在内存里。不写 SQLite，不进入 48 小时清理。

## 谁可以打给谁

只有 `direct` 会话显示「语音」和「视频」。群会话不显示这两个按钮。

服务在转发任何通话信令前确认：

- 会话存在，并且类型是 `direct`
- 发送者是该会话成员
- 信令里的通话只属于当前这一对成员

`callId` 由服务生成。客户端不能指定 `callId` 来加入别人的通话。

一个用户同时只在一路通话中。对方离线、任意一方已在通话中，或 30 秒内没有接听，服务向主叫发送 `call.reject`，原因分别是 `offline`、`busy`、`timeout`。被叫主动拒绝的原因是 `reject`。不是本会话成员的原因是 `forbidden`。

同一账号打开多个页面时，来电发到该用户的全部连接。任一页接听后，其余页面收到 `call.end` 并关掉来电层。后到的接听被忽略。

任一方 WebSocket 断开，服务把这一路通话视为挂断，并通知另一方。

## 信令

客户端发往服务：

| type | 字段 |
| --- | --- |
| `call.invite` | `conversationId`，`kind` 为 `audio` 或 `video` |
| `call.accept` | `callId` |
| `call.reject` | `callId` |
| `call.hangup` | `callId` |
| `call.offer` | `callId`，`sdp` |
| `call.answer` | `callId`，`sdp` |
| `call.candidate` | `callId`，`candidate` |

服务发往客户端：

| type | 字段 |
| --- | --- |
| `call.ringing` | `callId`，`conversationId`，`turn` |
| `call.invite` | `callId`，`conversationId`，`kind`，`fromUserId`，`fromNickname`，`turn` |
| `call.accept` | `callId` |
| `call.reject` | `callId`，`reason` |
| `call.hangup` | `callId` |
| `call.offer` | `callId`，`sdp` |
| `call.answer` | `callId`，`sdp` |
| `call.candidate` | `callId`，`candidate` |
| `call.end` | `callId` |

`turn` 包含 `urls`、`username` 和 `credential`。`urls` 同时给出指向本机的 `stun:` 和 `turn:` UDP 地址。凭据 5 分钟内有效，只够建立这一路通话。信令里不出现登录 Key 或管理员密码。

`kind` 为 `audio` 时只请求麦克风。`kind` 为 `video` 时同时请求麦克风和摄像头。

顺序是：主叫获得设备后发送 `call.invite`。被叫接受后，主叫才创建 offer。被叫返回 answer。双方随后交换 ICE candidate。被叫拒绝前，主叫不发送 offer。振铃期间任意一方发送 `call.hangup` 或 `call.reject`，服务都结束这路通话并通知另一方。

## 界面

单聊标题栏放两个按钮：「语音」和「视频」。颜色沿用现有主色 `#0B6BCB`、主文字 `#1F2329`、次文字 `#646A73`、面板白 `#FFFFFF` 和分割线 `#E6E8EB`。

来电盖在当前聊天页上，显示对方昵称、语音或视频，以及「接听」「拒绝」。

接通后使用同一块浮层。语音通话显示对方昵称和计时。视频通话显示双方画面，本地画面较小并叠在远端画面上。浮层提供静音、关闭摄像头和挂断。视频通话里关闭摄像头后，对方看到对方昵称，不保留最后一帧。

麦克风或摄像头被浏览器拒绝时，不发送 `call.invite`，浮层说明无法使用设备。连接失败时浮层显示「连接失败」，并发送 `call.hangup`。

窄屏仍使用现有的列表和聊天两屏。通话浮层盖在聊天屏上。

## 桌面程序

`desktop/src/banner.js` 输出 `https://` 地址。主进程访问 `https://127.0.0.1:<端口>/` 判断服务是否起来，并接受这张自签证书。管理页打开的地址同样使用 `https://`。

## 错误处理

- 证书写入失败时进程退出，并在终端说明原因。
- TURN 端口绑定失败时进程退出，避免通话没有中转通道却仍显示按钮。
- 未接听满 30 秒，服务清理这路通话并通知主叫 `timeout`。被叫来电层同时关闭。
- 挂断、拒绝、忙线、离线、超时和对端断开都会清除双方的占用状态。

## 测试

- 证书包含 `127.0.0.1` 和传入的局域网地址。地址集合变化时重新生成。私钥文件权限是 `0600`。
- 非成员、群会话和伪造的 `callId` 收不到对方的信令。
- 对方离线、任意一方已在通话、30 秒未接听，分别返回 `offline`、`busy`、`timeout`，之后双方都可以再次呼叫。
- 一方断开 WebSocket 后，另一方收到 `call.hangup`。
- `-tls=false` 时聊天页没有通话按钮。

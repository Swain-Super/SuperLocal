# 单聊屏幕共享设计

单聊增加「共享屏幕」。发起方把自己的屏幕或窗口共享给对方看，对方不能操作这台电脑。画面在两台浏览器之间用 WebRTC 直接传输。服务只转发信令。

本设计替换《一对一 WebRTC 通话设计》里「屏幕共享不在这一版」这一句。远程控制、剪贴板同步、系统声音、群共享和共享记录不做。

## 技术划分

| 部分 | 职责 |
| --- | --- |
| `internal/screen` | 屏幕共享状态、成员校验、信令转发、忙线和超时 |
| `internal/httpapi` | 接收 `screen.*`，断开连接时结束共享 |
| `web/src/screen.ts` | 浏览器侧 `RTCPeerConnection` 和 `getDisplayMedia` |
| `web/src/screenState.ts` | 共享浮层的状态 |
| `web/src/views/Chat.vue` | 单聊按钮和共享浮层 |

不新增前端依赖。画面使用浏览器内置的 `RTCPeerConnection` 和 `getDisplayMedia`。聊天页仍不使用 Element Plus。TURN 继续使用现有的 `github.com/pion/turn/v4`，凭据规则和通话相同。

屏幕共享状态只留在内存里。不写 SQLite，不进入 48 小时清理。

桌面程序不改。抓屏幕和显示画面都在聊天页里完成。

## 和通话的关系

屏幕共享和语音、视频各用各的会话。`internal/screen` 不读写 `internal/call` 的占用表。同一对用户可以同时有一路通话和一路屏幕共享。

一个用户同时只在一路屏幕共享中。对方或自己已经在共享时，新的请求得到 `busy`。正在通话不会让共享变成 `busy`，正在共享也不会让通话变成 `busy`。

挂断通话只结束通话。结束共享只结束共享。某个 WebSocket 关闭时，通话和共享各自按现有通话的规则结束：这一侧的会话通知另一侧，即使该用户还有别的页面连着。

屏幕共享和通话一起开关。`-tls=false` 时不创建 screen 会话，聊天页不显示「共享屏幕」，也不处理共享邀请。有通话能力时，`GET /api/features` 仍只返回 `{"calls": true}`，不另加字段。

## 谁可以共享

只有 `direct` 会话显示「共享屏幕」。群会话不显示。

按钮出现的条件和语音、视频相同：`calls` 为真，并且页面是安全上下文。

服务在转发任何共享信令前确认：

- 会话存在，并且类型是 `direct`
- 发送者是该会话成员
- 信令里的共享只属于当前这一对成员

`screenId` 由服务生成。客户端不能指定 `screenId` 来加入别人的共享。

发起方共享自己的屏幕。对方接受之后，发起方才选择屏幕或窗口。手机等没有 `getDisplayMedia` 的设备不能发起，仍可以接受并观看。

## 信令

客户端发往服务：

| type | 字段 |
| --- | --- |
| `screen.invite` | `conversationId` |
| `screen.accept` | `screenId` |
| `screen.reject` | `screenId` |
| `screen.hangup` | `screenId` |
| `screen.offer` | `screenId`，`sdp` |
| `screen.answer` | `screenId`，`sdp` |
| `screen.candidate` | `screenId`，`candidate` |

服务发往客户端：

| type | 字段 |
| --- | --- |
| `screen.ringing` | `screenId`，`conversationId`，`turn` |
| `screen.invite` | `screenId`，`conversationId`，`fromUserId`，`fromNickname`，`turn` |
| `screen.accept` | `screenId` |
| `screen.reject` | `screenId`，`reason` |
| `screen.hangup` | `screenId` |
| `screen.offer` | `screenId`，`sdp` |
| `screen.answer` | `screenId`，`sdp` |
| `screen.candidate` | `screenId`，`candidate` |
| `screen.end` | `screenId` |

`turn` 的内容和通话相同，包含 `urls`、`username` 和 `credential`。信令里不出现登录 Key 或管理员密码。

`reason` 只有这些值：`offline`、`busy`、`timeout`、`reject`、`forbidden`。

顺序是：

1. 发起方确认浏览器有 `getDisplayMedia` 之后发送 `screen.invite`。此时还没有画面。
2. 服务向发起方发送 `screen.ringing`，向对方发送 `screen.invite`。
3. 对方接受则发送 `screen.accept`。服务把 `screen.accept` 发给发起方，并把 `screen.end` 发给观看方的连接，用来关掉其他页面上的邀请。拒绝则发送 `screen.reject`。
4. 发起方收到 `screen.accept` 后显示「对方已接受」和「选择屏幕」。点击「选择屏幕」才调用 `getDisplayMedia`。浏览器要求这次调用来自用户点击，不能在收到信令时直接抓屏。只要画面，不要麦克风，也不要系统声音。视频约束优先使用 `cursor: "always"`，浏览器不接受该约束时去掉它再请求一次。
5. 选择成功后，发起方才发送 `screen.offer`。观看方返回 `screen.answer`。双方随后交换 ICE candidate。浏览器优先走局域网直连，直连失败时经现有 TURN 中转。不设置 `iceTransportPolicy`。
6. 只有发起方可以发送 offer，只有观看方可以发送 answer。未接受、伪造的 `screenId`，或身份不对的 offer、answer、candidate，服务不转发。

30 秒超时只覆盖对方尚未接受的阶段。接受之后，观看方停在「对方正在选择屏幕」，选择框开着多久都不算超时。计时使用服务注入的时钟，不直接读墙钟。

同一账号打开多个页面时，邀请发到观看方的全部连接。任一页接受后，其余页面收到 `screen.end` 并关掉邀请。发出邀请的那一页收到 `screen.accept` 后显示「选择屏幕」，由这一页的点击调用 `getDisplayMedia`。发起方的其他页面不弹选择框。接受邀请的那一页忽略紧随其后的 `screen.end`。

## 界面

单聊标题栏在「语音」和「视频」旁边放「共享屏幕」。正在共享时按钮仍可见。空闲时点击发起。已经在这路共享中时再点一次：收到邀请则拒绝，其余进行中的阶段则挂断。失败浮层还在时再点一次，只关掉浮层，不发送信令。

共享浮层使用和通话相同的深色底 `#1f2329`。接受按钮 `#0b6bcb`，拒绝和结束按钮 `#e11d48`。文字用白色和 `#e7f0fa`。

观看方的邀请显示对方昵称和「邀请你查看屏幕」，下面是「接受」和「拒绝」。

发起方发出请求后显示「等待对方接受」。对方接受后显示「对方已接受」和「选择屏幕」。点下去之后浏览器才弹出选择框。连上之后不显示自己屏幕的大画面，只显示「正在共享」和「结束共享」。

观看方在对方选择期间显示「对方正在选择屏幕」。画面到达后按比例完整放进浮层，使用 `object-fit: contain`，不裁切。两侧留白沿用浮层底色。按钮是「结束观看」。

只有通话时，通话浮层保持现在的样子。只有共享时，使用上面的共享浮层。两路同时进行时，屏幕占满画面，通话的静音、摄像头和挂断收成画面右下角的一排小按钮。摄像头按钮仍只在视频通话里出现。共享自己的结束按钮留在共享浮层上。

通话进行中收到共享邀请时，共享邀请盖在通话上面。拒绝、超时或结束后，通话还在。共享进行中收到通话邀请时，通话邀请盖在共享上面。拒绝或结束后，共享还在。两路邀请同时未处理时，后到的一层盖在上面。共享中接听通话后，改成「屏幕占满、通话按钮收在右下角」。

窄屏仍使用现有的列表和聊天两屏。共享浮层盖在聊天屏上。

失败时用「知道了」关闭浮层。发起方看到的文案：

| 情况 | 文案 |
| --- | --- |
| 对方已在别的共享中，或自己已在共享中 | 对方忙线 |
| 30 秒未接受 | 对方未接受 |
| 对方离线 | 对方离线 |
| 对方拒绝 | 对方已拒绝 |
| 不是单聊、不是成员，或服务拒绝 | 无法共享 |
| 选择框取消或系统拒绝屏幕权限 | 无法共享 |
| WebRTC 连接失败 | 连接失败 |

观看方在对方取消选择、权限失败或中途停止时看到「对方已结束共享」，然后用「知道了」关闭。发起方在观看方结束时看到「对方已结束观看」。发起方自己点「结束共享」，或浏览器停掉共享轨道时，发起方的浮层直接关闭，并发送 `screen.hangup`。观看方自己点「结束观看」时，观看方的浮层直接关闭。

没有 `getDisplayMedia` 时不打开浮层，也不发送 `screen.invite`。按钮旁提示「当前设备无法共享屏幕」。

`calls` 为假时忽略收到的 `screen.*`，不弹邀请。

## 错误处理

- 群聊、非成员和伪造的 `screenId` 得不到对方的信令，发起方收到 `forbidden`。
- 对方离线收到 `offline`。双方任一已在屏幕共享中收到 `busy`。
- 未接受满 30 秒，服务向双方发送 `screen.reject`，原因是 `timeout`，并清除占用。观看方只关掉邀请层。
- 接受之前到达的 offer 不转发。
- 选择框取消、权限被拒绝、连接失败、共享轨道结束，都发送 `screen.hangup`。
- 挂断通话不调用 screen 的挂断。结束共享不调用 call 的挂断。
- WebSocket 断开时分别结束这一侧的通话和屏幕共享。

## 测试

`internal/screen`：

- 群会话、非成员和伪造的 `screenId` 是 `forbidden`，对方收不到信令。
- 对方离线是 `offline`。
- 任一方已有屏幕共享是 `busy`。任一方已有语音或视频通话不是 `busy`。
- 30 秒未接受是 `timeout`，双方都可以再次发起。接受之后不再因为停留在选择屏幕而超时。
- 挂断通话不影响共享。结束共享不影响通话。
- 断开 WebSocket 后，另一方收到 `screen.hangup`。
- 接受前的 offer 不转发。接受后只有发起方的 offer 和观看方的 answer 会转发。

`web/src/screenState.ts`：

- 邀请、接受、拒绝、选择屏幕、连接中、正在共享和失败这些阶段。
- 发起请求的状态变化不依赖 `getDisplayMedia`。调用 `getDisplayMedia` 发生在收到接受之后。

## 不做

- 远程控制鼠标和键盘
- 剪贴板同步
- 系统声音和麦克风
- 群共享
- 共享记录
- 在共享画面上涂鸦

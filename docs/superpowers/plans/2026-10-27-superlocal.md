# SuperLocal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付一个 Go 二进制：启动后打印聊天页、管理后台和管理员账密，浏览器里用 Key 登录，进行一对一和群聊，并发送文字、图片和文件。

**Architecture:** Go 进程提供 HTTP、WebSocket 和 SQLite。聊天规则放在 `internal/store` 与 `internal/auth`。Vue 3 构建结果嵌入二进制，由同一进程托管。

**Tech Stack:** Go 1.22、`modernc.org/sqlite`、`golang.org/x/crypto/bcrypt`、`github.com/coder/websocket`、`github.com/oklog/ulid/v2`、Vue 3、Vue Router、Pinia、Axios、Vite、Element Plus（仅管理页）、UnoCSS。

## Global Constraints

- 监听默认 `:8080`，参数 `-addr`；数据目录默认 `./data`，参数 `-data`；单文件上限默认 `200MB`，参数 `-max-upload`。
- 消息和文件保留 48 小时。启动时清理一次，之后每 48 小时清理一次。保留时间不是配置项。
- 纯 Go SQLite，WAL，构建使用 `CGO_ENABLED=0`。
- `data/admin.secret` 权限 `0600`。管理员用户名 `admin`，初始密码 16 位大小写字母和数字。丢失该文件时重置密码。
- Cookie 名 `sl_session` 与 `sl_admin`，均为 HttpOnly、`SameSite=Lax`、有效期 7 天。
- Key 去掉首尾空格，长度 8–64，只允许字母、数字、`-` 和 `_`。同一 Key 即同一账号。
- 登录接口每个 IP 每分钟最多 20 次。
- 昵称 1–32 个字，不含换行。群名 1–40 个字。文字消息最长 4000 字。
- 头像仅 JPEG、PNG、WebP、GIF，最大 2MB。头像、用户、群成员关系不参与 48 小时清理。
- 图片消息 MIME 只允许 `image/jpeg`、`image/png`、`image/webp`、`image/gif`。普通文件不限制 MIME。
- 建群至少再选 1 名未封禁用户。创建者可加人，不能踢人。任何成员可退群。管理员解散群时删除成员关系。
- 一对一在第一条消息后才出现在会话列表。对方被封禁后不能再向该会话发送。
- 消息编号为 ULID。断线补齐使用 `GET /api/conversations/:id/messages?after=`，该接口不改未读。
- 未读等于最后阅读编号之后、且不是自己发送的消息数。打开会话时调用 `POST /api/conversations/:id/read`。
- 聊天接口、在线列表、搜索、会话、消息和 WebSocket 不返回别人的 Key。`GET /api/me` 返回自己的 Key。管理接口返回 Key。
- 下载文件必须是该会话当前成员。数据库锁最多重试 3 次，间隔 50 毫秒。
- 用户搜索最多 50 人。重连间隔 1 秒、2 秒、4 秒，上限 15 秒。
- 界面主色 `#0B6BCB`，按下 `#0958A8`，页面底 `#F3F5F7`，面板 `#FFFFFF`，边框 `#E6E8EB`，主文字 `#1F2329`，次文字 `#646A73`，在线 `#22A06B`，对方气泡 `#F2F3F5`。
- 正文字号 14px，辅助 12px，圆角 8px，头像 36px。字体 `PingFang SC`、`Microsoft YaHei`、`sans-serif`。产品名 SuperLocal。
- 视口窄于 960px 时列表和聊天分两屏。图片预览最宽 320px。Enter 发送，Shift+Enter 换行，输入法组字时 Enter 不发送。
- 聊天页不使用 Element Plus。不引入内部 npm 包，不使用 `nanoid`、`lodash`、`ant-design-vue`、`vuex`。
- 第一版不做屏幕共享、远程控制、全员广播、内置 HTTPS、修改 Key、群主踢人和群主转让。

## File Structure

| 路径 | 职责 |
| --- | --- |
| `cmd/superlocal/main.go` | 解析参数、初始化数据、打印地址和账密、启动清理和 HTTP |
| `internal/config/config.go` | 启动参数 |
| `internal/store/db.go` | 打开 SQLite、迁移、忙等重试 |
| `internal/store/id.go` | ULID |
| `internal/store/users.go` | 用户、公开字段 |
| `internal/store/sessions.go` | 登录会话 |
| `internal/store/chat.go` | 会话、消息、未读、权限 |
| `internal/store/files.go` | 文件元数据 |
| `internal/auth/key.go` | Key 校验 |
| `internal/auth/limit.go` | 登录限流 |
| `internal/auth/admin.go` | 管理员密码文件和哈希 |
| `internal/files/save.go` | 上传、头像、下载授权 |
| `internal/cleanup/cleanup.go` | 过期消息和临时文件 |
| `internal/hub/hub.go` | 连接、在线、踢下线、推送 |
| `internal/httpapi/*.go` | HTTP 与 WebSocket |
| `internal/web/embed.go` | 嵌入 `dist` |
| `web/` | Vue 聊天页和管理页 |

后续任务里的类型以这里为准，不要另起名字。

```go
type Config struct {
    Addr      string
    DataDir   string
    MaxUpload int64
}

type User struct {
    ID, Key, Nickname, AvatarPath string
    Banned, CanSendMessage, CanSendFile, CanCreateGroup bool
    LastSeenAt, CreatedAt time.Time
}

type PublicUser struct {
    ID, Nickname, AvatarURL string
    Online bool
}

type Me struct {
    ID, Key, Nickname, AvatarURL string
    CanSendMessage, CanSendFile, CanCreateGroup bool
}

func Public(u User) PublicUser
func MeView(u User) Me
```

`Public` 和 `MeView` 的 `AvatarURL` 在有头像时为 `/api/avatars/` + 用户编号，否则为空字符串。`Public` 的 JSON 没有 `key`。

---

### Task 1: 启动参数

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `config.FromArgs(args []string) (config.Config, error)`，字段 `Addr`、`DataDir`、`MaxUpload`

- [ ] **Step 1: Write the failing test**

```go
package config

import "testing"

func TestFromArgsDefaults(t *testing.T) {
    cfg, err := FromArgs([]string{})
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Addr != ":8080" || cfg.DataDir != "./data" || cfg.MaxUpload != 200*1024*1024 {
        t.Fatalf("%+v", cfg)
    }
}

func TestFromArgsOverride(t *testing.T) {
    cfg, err := FromArgs([]string{"-addr", "127.0.0.1:9", "-data", "/tmp/sl", "-max-upload", "10MB"})
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Addr != "127.0.0.1:9" || cfg.DataDir != "/tmp/sl" || cfg.MaxUpload != 10*1024*1024 {
        t.Fatalf("%+v", cfg)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -count=1`
Expected: FAIL，`FromArgs` 未定义

- [ ] **Step 3: Write minimal implementation**

`go.mod` 模块路径为 `superlocal`，Go `1.22`。

```go
package config

import (
    "flag"
    "fmt"
    "strconv"
    "strings"
)

type Config struct {
    Addr      string
    DataDir   string
    MaxUpload int64
}

func FromArgs(args []string) (Config, error) {
    fs := flag.NewFlagSet("superlocal", flag.ContinueOnError)
    addr := fs.String("addr", ":8080", "listen address")
    data := fs.String("data", "./data", "data directory")
    maxUpload := fs.String("max-upload", "200MB", "max upload size")
    if err := fs.Parse(args); err != nil {
        return Config{}, err
    }
    n, err := parseSize(*maxUpload)
    if err != nil {
        return Config{}, err
    }
    return Config{Addr: *addr, DataDir: *data, MaxUpload: n}, nil
}

func parseSize(raw string) (int64, error) {
    s := strings.TrimSpace(strings.ToUpper(raw))
    mult := int64(1)
    switch {
    case strings.HasSuffix(s, "MB"):
        mult = 1024 * 1024
        s = strings.TrimSuffix(s, "MB")
    case strings.HasSuffix(s, "KB"):
        mult = 1024
        s = strings.TrimSuffix(s, "KB")
    }
    n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
    if err != nil || n <= 0 {
        return 0, fmt.Errorf("无效的大小 %q", raw)
    }
    return n * mult, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod internal/config
git commit -m "$(cat <<'EOF'
feat(config): 解析监听地址、数据目录和上传上限

EOF
)"
```

---

### Task 2: SQLite 与用户编号

**Files:**
- Create: `internal/store/db.go`
- Create: `internal/store/id.go`
- Test: `internal/store/db_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `store.Open(path string) (*store.DB, error)`、`store.NewID(now time.Time) string`、`(*store.DB).SQL() *sql.DB`

- [ ] **Step 1: Write the failing test**

```go
package store

import (
    "testing"
    "time"
)

func TestOpenCreatesSchema(t *testing.T) {
    db := openTestDB(t)
    var name string
    err := db.SQL().QueryRow(`SELECT name FROM sqlite_master WHERE name = 'users'`).Scan(&name)
    if err != nil || name != "users" {
        t.Fatal(err, name)
    }
}

func TestNewIDOrdersByTime(t *testing.T) {
    a := NewID(time.UnixMilli(1_700_000_000_000))
    b := NewID(time.UnixMilli(1_700_000_000_100))
    if a >= b {
        t.Fatalf("%s >= %s", a, b)
    }
}
```

测试辅助函数 `openTestDB` 使用 `t.TempDir()` 下的 `app.db` 调用 `Open`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1 -run 'TestOpenCreatesSchema|TestNewIDOrdersByTime'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

执行 `go get modernc.org/sqlite github.com/oklog/ulid/v2`。

`Open` 使用 `modernc.org/sqlite`，DSN 加上 `_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)`。建表：

- `users(id, key UNIQUE, nickname, avatar_path, banned, can_send_message, can_send_file, can_create_group, last_seen_at, created_at)`
- `sessions(token_hash PRIMARY KEY, kind, user_id, expires_at)`
- `admin_user(username PRIMARY KEY, password_hash)`
- `conversations(id, type, name, owner_id, direct_key UNIQUE, dissolved, created_at)`
- `members(conversation_id, user_id, last_read_id, joined_at, PRIMARY KEY(conversation_id, user_id))`
- `messages(id, conversation_id, sender_id, kind, body, file_id, created_at)`
- `files(id, owner_id, name, mime, size, path, message_id, created_at)`

布尔值存 `INTEGER` 0/1。时间存 Unix 毫秒。`direct_key` 默认空字符串；空字符串不参与唯一约束时，用 `NULLIF` 无法在 SQLite 简单做到，因此一对一写入 `userA|userB`，群的 `direct_key` 写入该群自己的 id，保证唯一且不与一对一冲突。

`NewID` 用互斥锁保护 `ulid.Monotonic`。`(*DB).SQL()` 返回底层 `*sql.DB`。

忙等：提供 `(*DB).Exec` 和 `(*DB).QueryRow` 不强制。各写方法遇到 `database is locked` 时重试 3 次，间隔 50 毫秒。在 `db.go` 实现 `func (db *DB) withLockRetry(fn func() error) error`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -count=1 -run 'TestOpenCreatesSchema|TestNewIDOrdersByTime'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/store/db.go internal/store/id.go internal/store/db_test.go
git commit -m "$(cat <<'EOF'
feat(store): 初始化 SQLite 表和 ULID

EOF
)"
```

---

### Task 3: Key 与用户

**Files:**
- Create: `internal/auth/key.go`
- Create: `internal/store/users.go`
- Test: `internal/auth/key_test.go`
- Test: `internal/store/users_test.go`

**Interfaces:**
- Consumes: `store.Open`、`store.NewID`
- Produces: `auth.NormalizeKey(raw string) (string, error)`、`(*store.DB).LoginOrCreate(key, nickname string, now time.Time) (store.User, error)`、`(*store.DB).UserByID(id string) (store.User, error)`、`store.Public(u User) PublicUser`、`store.MeView(u User) Me`

- [ ] **Step 1: Write the failing test**

```go
func TestNormalizeKey(t *testing.T) {
    got, err := NormalizeKey("  ab_cd-12 ")
    if err != nil || got != "ab_cd-12" {
        t.Fatal(got, err)
    }
    if _, err := NormalizeKey("short"); err == nil {
        t.Fatal("expected error")
    }
    if _, err := NormalizeKey("has space!!"); err == nil {
        t.Fatal("expected error")
    }
}
```

```go
func TestLoginOrCreateAndHideKey(t *testing.T) {
    db := openTestDB(t)
    now := time.Unix(1_700_000_000, 0)
    first, err := db.LoginOrCreate("user-key1", "甲", now)
    if err != nil {
        t.Fatal(err)
    }
    second, err := db.LoginOrCreate("user-key1", "乙", now)
    if err != nil || second.Nickname != "甲" || second.ID != first.ID {
        t.Fatalf("%+v", second)
    }
    pub := Public(first)
    raw, _ := json.Marshal(pub)
    if strings.Contains(string(raw), "user-key1") || strings.Contains(string(raw), `"key"`) {
        t.Fatalf("public json leaked key: %s", raw)
    }
    me := MeView(first)
    if me.Key != "user-key1" {
        t.Fatal(me.Key)
    }
}
```

新用户未带昵称时返回错误。昵称含换行或超过 32 个字时返回错误。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ ./internal/store/ -count=1 -run 'TestNormalizeKey|TestLoginOrCreateAndHideKey'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`NormalizeKey`：`strings.TrimSpace`，再匹配 `^[A-Za-z0-9_-]{8,64}$`。

`LoginOrCreate`：已存在则返回原用户，不改昵称。不存在则校验昵称（trim、1–32 个 rune、不含 `\n` `\r`）后插入，三项权限默认为允许，`banned` 为 false。

`Public` 的 JSON 标签只有 `id`、`nickname`、`avatarUrl`、`online`。`Me` 的 JSON 标签为 `id`、`key`、`nickname`、`avatarUrl`、`canSendMessage`、`canSendFile`、`canCreateGroup`。新用户这三项默认为 true。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ ./internal/store/ -count=1 -run 'TestNormalizeKey|TestLoginOrCreateAndHideKey'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/key.go internal/auth/key_test.go internal/store/users.go internal/store/users_test.go
git commit -m "$(cat <<'EOF'
feat(auth): 用 Key 建立用户并隐藏他人 Key

EOF
)"
```

---

### Task 4: 会话 Cookie 的服务端记录

**Files:**
- Create: `internal/store/sessions.go`
- Test: `internal/store/sessions_test.go`

**Interfaces:**
- Consumes: `store.DB`
- Produces: `(*store.DB).CreateSession(kind, userID string, now time.Time) (token string, error)`、`(*store.DB).LookupSession(token string, now time.Time) (kind, userID string, error)`、`(*store.DB).DeleteSession(token string) error`

`kind` 只能是 `user` 或 `admin`。有效期 7 天。数据库保存 SHA-256 十六进制，不保存明文 token。

- [ ] **Step 1: Write the failing test**

创建用户会话，`LookupSession` 返回同一个 `userID`。把时钟拨到 8 天后，查找返回错误。`DeleteSession` 后立即查找返回错误。用明文 token 查询 `sessions` 表，结果行数为 0；用 SHA-256 十六进制查询，行数为 1。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1 -run TestSessions`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

token 为 `crypto/rand` 的 32 字节，编码成 hex。过期行在查找时删除。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -count=1 -run TestSessions`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/sessions.go internal/store/sessions_test.go
git commit -m "$(cat <<'EOF'
feat(store): 保存七天有效的会话摘要

EOF
)"
```

---

### Task 5: 管理员密码

**Files:**
- Create: `internal/auth/admin.go`
- Test: `internal/auth/admin_test.go`

**Interfaces:**
- Consumes: `store.DB`、`admin_user` 表
- Produces: `auth.EnsureAdmin(db *store.DB, dataDir string) (username, password string, error)`、`auth.CheckAdmin(db *store.DB, username, password string) error`、`auth.ChangeAdminPassword(db *store.DB, dataDir, password string) error`

- [ ] **Step 1: Write the failing test**

`EnsureAdmin` 在空库上创建 `admin`，密码长度 16 且匹配 `^[A-Za-z0-9]{16}$`。`data/admin.secret` 权限在非 Windows 上为 `0600`，内容两行：用户名、密码。`CheckAdmin` 用该密码成功，错误密码失败。删除 secret 文件后再次 `EnsureAdmin`，新密码可以通过 `CheckAdmin`，旧密码失败。`ChangeAdminPassword` 后文件和 `CheckAdmin` 同时变为新密码。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -count=1 -run 'TestEnsureAdmin|TestChangeAdminPassword'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

执行 `go get golang.org/x/crypto`。哈希使用 `bcrypt.DefaultCost`。写文件使用 `os.OpenFile` 配合 `O_CREATE|O_TRUNC|O_WRONLY` 和 `0600`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -count=1 -run 'TestEnsureAdmin|TestChangeAdminPassword'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/admin.go internal/auth/admin_test.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(auth): 生成并更新管理员密码文件

EOF
)"
```

---

### Task 6: 登录限流

**Files:**
- Create: `internal/auth/limit.go`
- Test: `internal/auth/limit_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `auth.NewLimiter() *auth.Limiter`、`(*auth.Limiter).Allow(ip string, now time.Time) bool`

- [ ] **Step 1: Write the failing test**

同一 IP 在同一分钟内前 20 次 `Allow` 为 true，第 21 次为 false。另一 IP 仍为 true。把时间推进 61 秒后，原 IP 再次为 true。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -count=1 -run TestLimiter`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

用 `map[string][]time.Time` 和互斥锁。保留 `now-1分钟` 之内的记录。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -count=1 -run TestLimiter`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/auth/limit.go internal/auth/limit_test.go
git commit -m "$(cat <<'EOF'
feat(auth): 按 IP 限制每分钟登录次数

EOF
)"
```

---

### Task 7: 一对一消息

**Files:**
- Create: `internal/store/chat.go`
- Test: `internal/store/chat_test.go`

**Interfaces:**
- Consumes: `User`、`NewID`
- Produces:
  - `(*DB).OpenDirect(selfID, peerID string, now time.Time) (string, error)` 返回会话 id
  - `(*DB).PostMessage(senderID, convID, kind, body, fileID string, now time.Time) (Message, error)`
  - `(*DB).ListConversations(userID string, online map[string]bool) ([]ConversationView, error)`
  - `type Message struct { ID, ConversationID, SenderID, SenderNickname, Kind, Body, FileID string; CreatedAt time.Time }`
  - `type ConversationView struct { ID, Type, Title string; Peer *PublicUser; MemberCount, Unread int; LastBody string; LastAt time.Time }`

- [ ] **Step 1: Write the failing test**

两个用户调用两次 `OpenDirect`，会话 id 相同，且与参数顺序无关。第一条文字消息之前，`ListConversations` 为空；发送后列表有 1 条，标题是对方昵称，JSON 不含对方 Key。同一对用户不会产生第二行 `conversations`。文字超过 4000 个字返回错误。非成员发送返回错误。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1 -run 'TestDirect|TestTextLimit|TestNonMember'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`direct_key` 为两个用户 id 按字符串排序后用 `|` 连接。`PostMessage` 在 `kind == "text"` 时要求 `utf8.RuneCountInString(body) <= 4000` 且 `body` trim 后非空。发送前确认发送者是成员、未被封禁、`CanSendMessage` 为 true。一对一的另一方 `Banned` 为 true 时返回错误。

`ListConversations` 对 `type = direct` 要求存在至少一条消息。`Peer` 使用 `Public`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -count=1 -run 'TestDirect|TestTextLimit|TestNonMember'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/chat.go internal/store/chat_test.go
git commit -m "$(cat <<'EOF'
feat(chat): 建立唯一的一对一会话和文字消息

EOF
)"
```

---

### Task 8: 群、退群、解散和未读

**Files:**
- Modify: `internal/store/chat.go`
- Modify: `internal/store/chat_test.go`

**Interfaces:**
- Consumes: Task 7 的 `PostMessage`、`ListConversations`
- Produces:
  - `(*DB).CreateGroup(ownerID, name string, memberIDs []string, now time.Time) (string, error)`
  - `(*DB).AddMembers(actorID, convID string, memberIDs []string, now time.Time) error`
  - `(*DB).Leave(userID, convID string) error`
  - `(*DB).Dissolve(convID string) error`
  - `(*DB).Messages(userID, convID, afterID string) ([]Message, error)`
  - `(*DB).MarkRead(userID, convID, messageID string) error`
  - `(*DB).SearchUsers(query string) ([]PublicUser, error)`

- [ ] **Step 1: Write the failing test**

覆盖以下行为：

- 群名为空、超过 40 字、没有其他成员、成员已封禁、创建者 `CanCreateGroup` 为 false，均返回错误。
- 创建成功后，创建者的会话列表立即出现该群，即使还没有消息。
- 创建者可以再添加未封禁用户。非创建者调用 `AddMembers` 返回错误。
- 成员 `Leave` 后列表不再包含该群，且不能 `PostMessage`。
- `Dissolve` 删除全部 `members` 行，消息行仍在。
- 用户 A 发言后，用户 B 的 `Unread` 为 1，A 自己的未读为 0。B 调用 `MarkRead` 传入该消息 id 后未读为 0。
- `Messages` 传入 `after` 只返回编号更大的消息，且不改变未读。
- `SearchUsers("甲")` 最多 50 条，结果 JSON 不含 `key`，不含已封禁用户。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1 -run 'TestGroup|TestUnread|TestSearchUsers'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`CreateGroup` 把创建者和去重后的成员写入 `members`。`owner_id` 保存创建者。`AddMembers` 要求 `actorID == owner_id` 且创建者仍是成员。`SearchUsers` 使用 `nickname LIKE`，`banned = 0`，`LIMIT 50`，返回 `Public`。

未读 SQL：`id > last_read_id AND sender_id != ?`。`last_read_id` 为空时，空字符串小于任何 ULID，因此不用特殊分支。`MarkRead` 只允许改自己的成员行，且消息必须属于该会话。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -count=1 -run 'TestGroup|TestUnread|TestSearchUsers'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/chat.go internal/store/chat_test.go
git commit -m "$(cat <<'EOF'
feat(chat): 支持建群、退群、解散和未读

EOF
)"
```

---

### Task 9: 封禁和发言权限

**Files:**
- Modify: `internal/store/users.go`
- Modify: `internal/store/chat.go`
- Modify: `internal/store/chat_test.go`

**Interfaces:**
- Consumes: `PostMessage`、`User`
- Produces: `(*DB).UpdateUserAdmin(id string, banned, canSend, canFile, canGroup bool) error`

- [ ] **Step 1: Write the failing test**

`Banned`、`CanSendMessage == false` 时 `PostMessage` 失败。`CanSendFile == false` 时 `kind` 为 `image` 或 `file` 失败，文字仍成功。对方被封禁后，一对一 `PostMessage` 失败。`UpdateUserAdmin` 能写回这四项。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -count=1 -run TestPermissions`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

在 `PostMessage` 开头读取发送者。文件类消息额外检查 `CanSendFile`。一对一查询另一名成员的 `banned`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -count=1 -run TestPermissions`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/users.go internal/store/chat.go internal/store/chat_test.go
git commit -m "$(cat <<'EOF'
feat(chat): 按封禁和权限拒绝发送

EOF
)"
```

---

### Task 10: 文件、图片和头像

**Files:**
- Create: `internal/store/files.go`
- Create: `internal/files/save.go`
- Test: `internal/files/save_test.go`

**Interfaces:**
- Consumes: `PostMessage`、`Config.MaxUpload` 的数值由调用方传入
- Produces:
  - `(*DB).InsertFile(ownerID, name, mime, path string, size int64, now time.Time) (string, error)`
  - `(*DB).BindFile(fileID, messageID, ownerID string) error`
  - `(*DB).FileVisible(userID, fileID string) (store.File, error)`
  - `files.SaveUpload(dir, ownerID, filename, mime string, r io.Reader, max int64, imageOnly bool) (idPath string, size int64, err error)` 只写磁盘并返回相对路径
  - `files.SaveAvatar(dir, userID, mime string, r io.Reader) (relative string, error)`

`store.File` 字段：`ID, OwnerID, Name, Mime, Path, MessageID string`，`Size int64`，`CreatedAt time.Time`。

- [ ] **Step 1: Write the failing test**

普通文件超过 `max` 时返回错误且磁盘上没有残留。图片 MIME 为 `text/plain` 且 `imageOnly` 为 true 时失败。合法 PNG 写入 `files/` 子目录。`BindFile` 后，会话成员 `FileVisible` 成功；退群成员失败；未绑定消息的文件对其他人失败。头像超过 2MB 失败。允许的头像类型与图片消息相同。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/files/ ./internal/store/ -count=1 -run 'TestSaveUpload|TestVisibility|TestAvatar'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`SaveUpload` 先把内容写入 `dir/tmp/`，完成后移动到 `dir/files/<id>`。失败时删除临时文件。用 `io.LimitReader(r, max+1)` 判断超限。`PostMessage` 在 `kind` 为 `image` 或 `file` 时调用 `BindFile`：文件必须属于发送者且 `message_id` 仍为空。图片消息再检查 MIME。

头像写入 `dir/avatars/<userID>`，并更新 `users.avatar_path`。这一步由 `(*DB).SetAvatar(userID, relative string) error` 完成。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/files/ ./internal/store/ -count=1 -run 'TestSaveUpload|TestVisibility|TestAvatar'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/files.go internal/files
git commit -m "$(cat <<'EOF'
feat(files): 保存上传文件并按会话成员授权下载

EOF
)"
```

---

### Task 11: 48 小时清理

**Files:**
- Create: `internal/cleanup/cleanup.go`
- Test: `internal/cleanup/cleanup_test.go`

**Interfaces:**
- Consumes: `messages`、`files`、磁盘路径
- Produces: `cleanup.Run(db *store.DB, dataDir string, now time.Time) error`

- [ ] **Step 1: Write the failing test**

插入一条 49 小时前的消息及其文件，和一条 1 小时前的消息及其文件。再放一个 49 小时前、`message_id` 为空的临时文件记录。`Run` 之后，过期消息、过期文件和临时文件被删除，未过期的保留。用户行和头像文件仍在。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cleanup/ -count=1 -run TestRetention`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

删除 `messages.created_at < now-48h`。删除这些消息引用的文件记录和磁盘文件。删除 `message_id = '' AND created_at < now-48h` 的文件。不删除 `users` 和 `avatars`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cleanup/ -count=1 -run TestRetention`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cleanup
git commit -m "$(cat <<'EOF'
feat(cleanup): 删除超过 48 小时的消息和文件

EOF
)"
```

---

### Task 12: 聊天与管理员 HTTP

**Files:**
- Create: `internal/httpapi/server.go`
- Create: `internal/httpapi/cookies.go`
- Create: `internal/httpapi/auth_handlers.go`
- Create: `internal/httpapi/admin_handlers.go`
- Test: `internal/httpapi/auth_test.go`

**Interfaces:**
- Consumes: `LoginOrCreate`、`CreateSession`、`LookupSession`、`EnsureAdmin`、`Limiter`、`UpdateUserAdmin`
- Produces: `httpapi.New(deps httpapi.Deps) http.Handler`

```go
type Deps struct {
    DB        *store.DB
    Hub       *hub.Hub
    DataDir   string
    MaxUpload int64
    Limiter   *auth.Limiter
    Now       func() time.Time
}
```

Task 12 的 `Hub` 允许为 nil。路由先注册认证和管理员接口。

- [ ] **Step 1: Write the failing test**

使用 `httptest.Server`：

- `POST /api/session` 体 `{"key":"user-key1","nickname":"甲"}` 返回 200，`Set-Cookie` 名是 `sl_session`，且包含 `HttpOnly` 和 `SameSite=Lax`。
- 随后 `GET /api/me` 的 JSON 含自己的 key。
- 第 21 次错误登录尝试返回 429。
- 封禁该用户后再 `POST /api/session` 返回 403，正文含「封禁」。
- `POST /api/admin/session` 使用 `EnsureAdmin` 打印出的密码返回 `sl_admin`。错误密码返回 401，正文同时不含「密码错误」和「账号错误」，只含「登录失败」。
- 没有 `sl_admin` 时 `GET /api/admin/stats` 返回 401。
- 带聊天 Cookie 访问 `GET /api/admin/stats` 仍然 401。
- 带管理员 Cookie 时 `PATCH /api/admin/users/{id}` 可把 `canSendMessage` 设为 false，再次 `GET /api/admin/users` 能看到该用户的 key、`lastSeenAt` 和在线状态。
- `POST /api/admin/password` 体 `{"password":"new-password-16"}` 成功后，旧密码失效，当前 `sl_admin` 仍可访问。
- `GET /api/admin/groups` 返回群名、人数和创建时间。`DELETE /api/admin/groups/{id}` 后成员关系消失。
- 登录限流计算每一次 `POST /api/session`，成功的请求也计入 20 次。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi/ -count=1 -run 'TestSession|TestAdmin'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

错误 JSON 统一为 `{"error":"..."}`。聊天登录成功写 `sl_session`，`Path=/`，`MaxAge=604800`。管理员登录写 `sl_admin`。`POST /api/logout` 与 `POST /api/admin/logout` 删除对应会话并清空 Cookie。

`GET /api/admin/stats` 返回 `users`、`online`、`banned`、`messages`、`fileBytes`。`online` 在 Hub 为 nil 时返回 0。`messages` 与 `fileBytes` 只统计 48 小时内的记录。

`PATCH` 体为 `{"banned":bool,"canSendMessage":bool,"canSendFile":bool,"canCreateGroup":bool}`。封禁时若 Hub 非 nil，调用 `Kick`。`POST /api/admin/password` 调用 `ChangeAdminPassword`，不删除当前会话。`GET /api/admin/users` 的每一项包含 `id`、`key`、`nickname`、`online`、`lastSeenAt`、`banned`、`canSendMessage`、`canSendFile`、`canCreateGroup`。`GET /api/admin/groups` 的每一项包含 `id`、`name`、`memberCount`、`createdAt`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpapi/ -count=1 -run 'TestSession|TestAdmin'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "$(cat <<'EOF'
feat(api): 提供用户登录和管理员鉴权

EOF
)"
```

---

### Task 13: 聊天、上传和下载 HTTP

**Files:**
- Create: `internal/httpapi/chat_handlers.go`
- Create: `internal/httpapi/file_handlers.go`
- Modify: `internal/httpapi/server.go`
- Test: `internal/httpapi/chat_test.go`

**Interfaces:**
- Consumes: Task 7–10 的 store 和 files 函数、Task 12 的 Cookie
- Produces: 规格中的聊天与文件路由，外加 `POST /api/conversations/{id}/read`

- [ ] **Step 1: Write the failing test**

两个测试用户登录。甲 `POST /api/conversations/direct` 打开与乙的会话，再 `POST /api/files` 上传一个小文本，然后需要一条消息。此任务的文字消息由测试直接调用 `PostMessage`，文件消息也调用 `PostMessage`，因为 WebSocket 在下一任务。测试断言：

- `GET /api/users/online` 在 Hub 为 nil 时返回空数组，元素类型不会包含 key。
- `GET /api/users?q=乙` 返回乙的昵称且 JSON 不含乙的 key。
- `POST /api/groups` 创建群。非成员 `GET /api/files/{id}` 返回 403。
- 成员下载返回文件字节。
- `POST /api/me/avatar` 用 PNG 魔数字节成功，`GET /api/me` 的 `avatarUrl` 以 `/api/avatars/` 开头。`GET /api/avatars/{id}` 在已登录时返回该图片。
- `POST /api/conversations/{id}/read` 后，对方视角的未读仍可由 `GET /api/conversations` 读到；自己的未读变为 0。

PNG 测试字节使用 1x1 PNG 的完整最小文件，写在测试文件的原始字符串或字节切片里。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi/ -count=1 -run 'TestChatHTTP|TestFileHTTP'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

上传字段名使用 `file`。响应 `{"id":"...","name":"...","size":0,"mime":"..."}`。下载前调用 `FileVisible`。头像下载只要求已登录，不要求同会话。

`GET /api/conversations/{id}/messages` 查询参数 `after`。响应消息对象包含 `senderNickname`，不包含发送者 key。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpapi/ -count=1 -run 'TestChatHTTP|TestFileHTTP'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi
git commit -m "$(cat <<'EOF'
feat(api): 提供会话、上传和成员下载

EOF
)"
```

---

### Task 14: WebSocket

**Files:**
- Create: `internal/hub/hub.go`
- Create: `internal/httpapi/ws.go`
- Modify: `internal/httpapi/server.go`
- Test: `internal/hub/hub_test.go`
- Test: `internal/httpapi/ws_test.go`

**Interfaces:**
- Consumes: `PostMessage`、会话 Cookie
- Produces:
  - `hub.New() *hub.Hub`
  - `(*hub.Hub).Add(userID string, c *websocket.Conn)`
  - `(*hub.Hub).Remove(userID string, c *websocket.Conn) (nowOffline bool)`
  - `(*hub.Hub).OnlineIDs() map[string]bool`
  - `(*hub.Hub).Send(userIDs []string, payload []byte)`
  - `(*hub.Hub).Kick(userID string)`

执行 `go get github.com/coder/websocket`。

- [ ] **Step 1: Write the failing test**

`hub_test`：同一用户两连接都 `Online`。移除一条后仍在线，移除最后一条后离线。`Kick` 向该用户连接写入 `{"type":"session.revoked"}`。

`ws_test`：登录后连接 `GET /api/ws`。发送 `{"type":"message.send","conversationId":"...","kind":"text","text":"你好"}`。对方连接收到 `message.new`，其中没有对方 key，有发送者昵称。发送者自己也收到 `message.new`。无发言权时收到 `{"type":"error"}`，且 `Messages` 条数不增加。管理员封禁后，已连接的套接字收到 `session.revoked`。连接和断开会让其他在线用户收到 `presence`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hub/ ./internal/httpapi/ -count=1 -run 'TestHub|TestWS'`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`/api/ws` 只接受 `sl_session`。读到 `message.send` 后调用 `PostMessage`。成功后向当前成员推送：

```json
{"type":"message.new","message":{"id":"","conversationId":"","senderId":"","senderNickname":"","kind":"text","body":"","fileId":"","createdAt":""}}
```

失败时只向发送连接写 `{"type":"error","error":"..."}`。`Kick` 先写 `session.revoked` 再关闭。`PATCH` 封禁用户时调用 `Kick`。`ListConversations` 和在线用户接口使用 `OnlineIDs()`。用户连上或最后一条连接断开时，向所有在线用户推送 `{"type":"presence","userId":"","online":true}`。`last_seen_at` 在断开时更新。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/hub/ ./internal/httpapi/ -count=1 -run 'TestHub|TestWS'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hub internal/httpapi go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(ws): 通过 WebSocket 收发消息和在线状态

EOF
)"
```

---

### Task 15: 启动程序

**Files:**
- Create: `cmd/superlocal/main.go`
- Create: `internal/httpapi/static.go`
- Create: `internal/web/dist/index.html`
- Create: `internal/web/embed.go`
- Test: `cmd/superlocal/banner_test.go`

**Interfaces:**
- Consumes: `config.FromArgs`、`store.Open`、`auth.EnsureAdmin`、`cleanup.Run`、`httpapi.New`
- Produces: `main.banner(port, username, password string, addrs []string) string`、可执行文件 `superlocal`

- [ ] **Step 1: Write the failing test**

`banner(":8080", "admin", "secret", []string{"192.168.1.8"})` 的结果包含 `http://127.0.0.1:8080/`、`http://127.0.0.1:8080/admin`、`http://192.168.1.8:8080/`、`http://192.168.1.8:8080/admin`、账号 `admin` 和密码 `secret`。`main` 从 `net.InterfaceAddrs` 收集非回环 IPv4，再传给 `banner`。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/superlocal/ -count=1 -run TestBanner`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

`main` 创建数据目录，打开数据库，调用 `EnsureAdmin`，启动时执行一次 `cleanup.Run`，然后 `time.NewTicker(48 * time.Hour)` 再执行。向 stdout 打印 banner。HTTP 服务器使用 `httpapi.New`。

`internal/web/embed.go`：

```go
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
```

仓库提交占位文件 `internal/web/dist/index.html`，内容只显示 SuperLocal。`.gitignore` 增加 `internal/web/dist/assets/`。Vite 的 `emptyOutDir` 保持 false，避免构建时删掉占位页以外的嵌入入口；构建前由 Vite 覆盖 `index.html` 并写入 `assets`。本地 `go build` 前必须先 `npm run build`，二进制才会带上正式页面。`go test` 在只有占位页时也能通过。

`static.go` 对非 `/api/` 且磁盘上不存在的路径返回 `dist/index.html`。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/superlocal internal/httpapi/static.go internal/web/embed.go internal/web/dist/index.html .gitignore
git commit -m "$(cat <<'EOF'
feat(server): 启动服务并打印地址和账密

EOF
)"
```

---

### Task 16: 前端工程与登录页

**Files:**
- Create: `web/package.json`、`web/index.html`、`web/vite.config.ts`、`web/uno.config.ts`、`web/src/main.ts`、`web/src/router.ts`、`web/src/api.ts`、`web/src/views/Login.vue`、`web/src/views/admin/AdminLogin.vue`

**Interfaces:**
- Consumes: `POST /api/session`、`GET /api/me`、`POST /api/admin/session`
- Produces: `npm run build` 输出到 `internal/web/dist`

- [ ] **Step 1: 创建工程**

在 `web/` 执行非交互脚手架，并安装依赖，不手写版本号：

```bash
npm create vite@latest web -- --template vue-ts
cd web
npm install vue-router pinia axios element-plus
npm install -D unocss @unocss/reset
```

若目录已存在，在空的 `web/` 中执行上述命令。`vite.config.ts` 的 `build.outDir` 设为 `../internal/web/dist`，`emptyOutDir` 设为 false。路由：`/login`、`/`、`/admin/login`、`/admin`。

- [ ] **Step 2: 写登录页**

聊天登录卡片包含产品名 SuperLocal、说明「用 Key 进入，Key 即账号」、Key、昵称和「进入」。Key 在提交前用与后端相同的规则检查，不合格时不发请求。管理登录只有账号、密码和「登录」。失败时显示接口返回的 `error`。成功后分别进入 `/` 和 `/admin`。

颜色使用 Global Constraints 中的色值，写成 UnoCSS theme，而不是散落的随意色值。

- [ ] **Step 3: 验证构建**

Run: `cd web && npm run build`
Expected: 生成 `internal/web/dist/index.html`

- [ ] **Step 4: Commit**

```bash
git add web internal/web/dist package.json
git commit -m "$(cat <<'EOF'
feat(web): 添加聊天和管理员登录页

EOF
)"
```

只提交锁文件和源码，不提交 `internal/web/dist/assets`。

---

### Task 17: 聊天界面

**Files:**
- Create: `web/src/views/Chat.vue`
- Create: `web/src/stores/chat.ts`
- Modify: `web/src/api.ts`
- Modify: `web/src/router.ts`

**Interfaces:**
- Consumes: 规格中的聊天 REST 和 WebSocket 事件
- Produces: 三栏聊天页

- [ ] **Step 1: 写状态和 API**

`api.ts` 使用 Axios，`withCredentials: true`，`baseURL` 为空。封装 `me`、`logout`、`onlineUsers`、`searchUsers`、`conversations`、`openDirect`、`createGroup`、`addMembers`、`leave`、`messages`、`markRead`、`upload`、`patchMe`、`uploadAvatar`。

WebSocket 地址由 `location.protocol` 决定 `ws` 或 `wss`，路径 `/api/ws`。断线后按 1、2、4 秒递增，上限 15 秒。重连后对已打开会话调用 `messages?after=`。

- [ ] **Step 2: 写界面**

`Chat.vue` 实现规格「界面」一节：

- 顶栏显示自己的头像、昵称、在线点，以及个人资料。资料里 Key 只读，可改昵称和头像。
- 左栏两个按钮：「消息」「联系人」。
- 消息列表显示摘要、时间和未读数。离线头像使用 CSS `filter: grayscale(1)`。
- 联系人只显示在线用户。顶部「建群」在 `canCreateGroup` 为 false 时不渲染。建群对话框搜索已注册用户并多选。
- 聊天区宽度、气泡颜色、文件横条、图片最大宽度 320px 按规格。
- Enter 发送，Shift+Enter 换行。`compositionstart` 到 `compositionend` 期间忽略 Enter。
- 拖放文件到聊天区、联系人头像或群列表行时调用上传，再发 `message.send`。
- 窄于 960px 时用一个 `showThread` 状态切换列表和聊天，聊天页有「返回」。

- 聊天区顶部提供「添加成员」（仅群主且自己仍在群内）和「退出群」。添加成员复用建群的搜索。退出后回到列表。
- 上传图片 MIME 属于 jpeg、png、webp、gif 时，`message.send` 的 `kind` 为 `image`，否则为 `file`。
- 收到 `session.revoked` 后关闭连接并显示封禁说明。
- 打开会话时调用 `markRead`，并在当前会话收到 `message.new` 时再次调用。
- `GET /api/me` 的三项权限为 false 时，隐藏建群按钮，并禁用输入框或上传按钮。

- [ ] **Step 3: 类型检查和构建**

Run: `cd web && npm run build`
Expected: 退出码 0

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "$(cat <<'EOF'
feat(web): 实现飞鸽式三栏聊天界面

EOF
)"
```

---

### Task 18: 管理后台界面

**Files:**
- Create: `web/src/views/admin/Layout.vue`
- Create: `web/src/views/admin/Overview.vue`
- Create: `web/src/views/admin/Users.vue`
- Create: `web/src/views/admin/Groups.vue`

**Interfaces:**
- Consumes: `/api/admin/stats`、`/api/admin/users`、`/api/admin/groups`、`PATCH /api/admin/users/{id}`、`DELETE /api/admin/groups/{id}`、`POST /api/admin/password`
- Produces: 管理页三项目

- [ ] **Step 1: 只在管理页引入 Element Plus**

在 `Layout.vue` 和各管理视图中按需引入组件，不在 `main.ts` 全局 `app.use(ElementPlus)`。左侧导航宽 200px，项目为「概览」「用户」「群」。概览五张卡：注册用户、当前在线、封禁、保留期内消息、文件占用。用户表含昵称、Key、在线、最近在线、三个权限开关和封禁。群表可解散。提供修改管理员密码的表单。

- [ ] **Step 2: 构建**

Run: `cd web && npm run build && CGO_ENABLED=0 go test ./... -count=1`
Expected: 两者都成功

- [ ] **Step 3: Commit**

```bash
git add web/src
git commit -m "$(cat <<'EOF'
feat(web): 实现用户、群和概览管理页

EOF
)"
```

---

### Task 19: 手工验收

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: 已构建的二进制
- Produces: README 中的启动和验收步骤

- [ ] **Step 1: 写 README**

用中文写明：`cd web && npm run build`，然后 `CGO_ENABLED=0 go build -o superlocal ./cmd/superlocal`，再 `./superlocal`。说明终端里的聊天地址、管理地址、账号和密码。说明 Key 即账号，以及消息保留 48 小时。

- [ ] **Step 2: 按规格手工检查**

启动程序，核对打印。用两个浏览器配置文件完成：Key 登录、修改昵称和头像、确认看不到对方 Key、一对一文字、图片、文件、拖放文件、建群、退群、未读、离线头像变灰、封禁后连接断开、管理员改密后用新密码登录。把窗口缩到 960px 以下，确认列表和聊天分两屏。

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs: 说明 SuperLocal 的构建和启动方式

EOF
)"
```

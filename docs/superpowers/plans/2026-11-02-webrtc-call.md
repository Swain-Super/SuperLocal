# 一对一 WebRTC 通话 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 单聊可以发起语音和视频通话，音视频走 WebRTC，信令仍走现有 WebSocket。

**Architecture:** 服务默认在原端口提供 HTTPS，并在同一端口号的 UDP 上提供 TURN。`internal/call` 在内存里维护一路通话的状态和转发。浏览器只在安全上下文且服务声明开启通话时显示按钮，接听之后才交换 SDP。

**Tech Stack:** Go 1.22、`crypto/tls`、`github.com/pion/turn/v4`、现有 `github.com/coder/websocket`、Vue 3、浏览器 `RTCPeerConnection`。不新增前端依赖。

## Global Constraints

- 文字、图片、文件和在线状态仍由服务转发，不改成 WebRTC。
- 只有 `direct` 会话显示「语音」和「视频」。群通话、屏幕共享、通话记录不做。
- 服务默认 HTTPS。`-tls=false` 时只提供 HTTP，不监听 TURN，聊天页没有通话按钮。
- 证书是 ECDSA P-256，有效期两年，文件是 `data/tls/cert.pem` 和权限 `0600` 的 `data/tls/key.pem`。使用者可选名称包含 `localhost`、`127.0.0.1` 和启动时的局域网 IPv4。缺失、过期或地址集合变化时重新生成。
- TURN 使用和 HTTPS 相同的端口号，协议是 UDP。凭据 5 分钟有效。信令不出现登录 Key 或管理员密码。
- `callId` 由服务生成。一个用户同时只在一路通话中。30 秒未接听原因是 `timeout`。
- `kind` 为 `audio` 时只请求麦克风，为 `video` 时同时请求麦克风和摄像头。被叫接受后主叫才创建 offer。
- 颜色只用 `#0B6BCB`、`#1F2329`、`#646A73`、`#FFFFFF`、`#E6E8EB`。聊天页不使用 Element Plus。
- 桌面程序的地址和健康检查使用 `https://`，健康检查接受这张自签证书。
- `CGO_ENABLED=0`。设备字段继续用 `omvd`，本功能不新增设备字段。

## File Structure

| 路径 | 职责 |
| --- | --- |
| `internal/tlscert/cert.go` | 生成和加载自签证书 |
| `internal/config/config.go` | `-tls` 开关，默认 true |
| `internal/call/session.go` | 通话状态、忙线、超时、信令转发 |
| `internal/turncred/cred.go` | 5 分钟 TURN 用户名和密码 |
| `internal/turnserver/server.go` | 绑定 UDP 并启动 pion TURN |
| `internal/httpapi/call.go` | 把 WebSocket 通话事件交给 `call.Session` |
| `web/src/callState.ts` | 与浏览器无关的通话阶段 |
| `web/src/call.ts` | `getUserMedia` 和 `RTCPeerConnection` |
| `web/src/views/Chat.vue` | 按钮、来电层、通话浮层 |
| `desktop/src/banner.js` | 地址改为 `https://` |
| `desktop/src/main.js` | 健康检查和打开页面使用 HTTPS |
| `cmd/superlocal/main.go` | 按配置启动 HTTPS 和 TURN |

---

### Task 1: 自签证书

**Files:**
- Create: `internal/tlscert/cert.go`
- Test: `internal/tlscert/cert_test.go`

**Interfaces:**
- Consumes: 数据目录和 IP 字符串
- Produces: `func LoadOrCreate(dir string, ips []string, now time.Time) (tls.Certificate, error)`

- [ ] **Step 1: Write the failing test**

```go
package tlscert

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadOrCreateIncludesHostsAndKeyMode(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	cert, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Fatalf("algorithm %s", leaf.PublicKeyAlgorithm)
	}
	for _, name := range []string{"localhost", "127.0.0.1", "192.168.1.8"} {
		if err := leaf.VerifyHostname(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "tls", "key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", info.Mode().Perm())
	}
	again, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cert.Leaf == nil && len(again.Certificate) == 0 {
		t.Fatal("missing certificate")
	}
	if !sameCert(cert, again) {
		t.Fatal("reloaded a different certificate")
	}
}

func TestLoadOrCreateRegeneratesWhenAddressSetChanges(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	first, err := LoadOrCreate(dir, []string{"192.168.1.8"}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreate(dir, []string{"192.168.1.9"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if sameCert(first, second) {
		t.Fatal("address change reused the old certificate")
	}
	leaf, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := leaf.VerifyHostname("192.168.1.9"); err != nil {
		t.Fatal(err)
	}
}

func sameCert(a, b tls.Certificate) bool {
	if len(a.Certificate) == 0 || len(b.Certificate) == 0 {
		return false
	}
	return string(a.Certificate[0]) == string(b.Certificate[0])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tlscert/ -count=1`

Expected: FAIL with `undefined: LoadOrCreate`

- [ ] **Step 3: Write minimal implementation**

```go
package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func LoadOrCreate(dir string, ips []string, now time.Time) (tls.Certificate, error) {
	tlsDir := filepath.Join(dir, "tls")
	certPath := filepath.Join(tlsDir, "cert.pem")
	keyPath := filepath.Join(tlsDir, "key.pem")
	want := hostSet(ips)
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil && covers(leaf, want, now) {
			return cert, nil
		}
	}
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return tls.Certificate{}, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "SuperLocal"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  hostIPs(want),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func hostSet(ips []string) []string {
	seen := map[string]struct{}{"localhost": {}, "127.0.0.1": {}}
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			seen[ip] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for host := range seen {
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

func hostIPs(hosts []string) []net.IP {
	var out []net.IP
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			out = append(out, ip)
		}
	}
	return out
}

func covers(leaf *x509.Certificate, hosts []string, now time.Time) bool {
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return false
	}
	for _, host := range hosts {
		if err := leaf.VerifyHostname(host); err != nil {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tlscert/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tlscert/cert.go internal/tlscert/cert_test.go
git commit -m "$(cat <<'EOF'
feat(tls): 生成本地 HTTPS 证书

EOF
)"
```

---

### Task 2: 默认 HTTPS 和通话开关

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/superlocal/main.go`
- Modify: `cmd/superlocal/banner_test.go`
- Modify: `internal/httpapi/server.go`
- Create: `internal/httpapi/features_test.go`

**Interfaces:**
- Consumes: `tlscert.LoadOrCreate`
- Produces: `Config.TLS bool`，默认 `true`。`banner(port, username, password string, addrs []string, secure bool) string`。`GET /api/features` 返回 `{"calls":true}` 或 `{"calls":false}`。`Deps.CallsEnabled bool`。

- [ ] **Step 1: Write the failing test**

在 `internal/config/config_test.go` 的 `TestFromArgsDefaults` 里增加 `cfg.TLS` 必须为 `true`。追加：

```go
func TestFromArgsTLSOff(t *testing.T) {
	cfg, err := FromArgs([]string{"-tls=false"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TLS {
		t.Fatal("tls still on")
	}
}
```

把 `cmd/superlocal/banner_test.go` 的调用改成 `banner(":8080", "admin", "secret", []string{"192.168.1.8"}, true)`，期望字符串里的 `http://` 全部改成 `https://`。

新建 `internal/httpapi/features_test.go`：

```go
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/httpapi"
)

func TestFeaturesReflectCalls(t *testing.T) {
	db := openHTTPDB(t)
	for _, enabled := range []bool{true, false} {
		srv := httptest.NewServer(httpapi.New(httpapi.Deps{
			DB:           db,
			DataDir:      t.TempDir(),
			MaxUpload:    1024,
			Limiter:      auth.NewLimiter(),
			Now:          func() time.Time { return time.Unix(1_700_000_000, 0) },
			CallsEnabled: enabled,
		}))
		res, err := http.Get(srv.URL + "/api/features")
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Calls bool `json:"calls"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		srv.Close()
		if body.Calls != enabled {
			t.Fatalf("calls %v, want %v", body.Calls, enabled)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ ./cmd/superlocal/ ./internal/httpapi/ -count=1 -run 'TestFromArgs|TestBanner|TestFeaturesReflectCalls'`

Expected: FAIL，编译错误指出 `Config` 没有 `TLS`，`banner` 参数数量不对，`CallsEnabled` 未知。

- [ ] **Step 3: Write minimal implementation**

`config.go` 的 `Config` 增加 `TLS bool`。`FromArgs` 增加 `tlsOn := fs.Bool("tls", true, "serve HTTPS")`，返回值带上 `TLS: *tlsOn`。

`banner` 增加 `secure bool`。`secure` 为 true 时格式串用 `https`，否则用 `http`。

`main.go` 在创建 handler 前：

```go
callsEnabled := cfg.TLS
if cfg.TLS {
	if _, err := tlscert.LoadOrCreate(cfg.DataDir, lanIPv4(), time.Now()); err != nil {
		log.Fatalf("证书写入失败: %v", err)
	}
}
fmt.Print(banner(cfg.Addr, username, password, lanIPv4(), cfg.TLS))
handler := httpapi.WithStatic(httpapi.New(httpapi.Deps{
	DB:           db,
	Hub:          hub.New(),
	DataDir:      cfg.DataDir,
	MaxUpload:    cfg.MaxUpload,
	Limiter:      auth.NewLimiter(),
	CallsEnabled: callsEnabled,
}))
if cfg.TLS {
	cert, err := tlscert.LoadOrCreate(cfg.DataDir, lanIPv4(), time.Now())
	if err != nil {
		log.Fatalf("证书写入失败: %v", err)
	}
	ln, err := tls.Listen("tcp", cfg.Addr, &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		log.Fatal(err)
	}
	err = http.Serve(ln, handler)
} else {
	err = http.ListenAndServe(cfg.Addr, handler)
}
```

`server` 和 `Deps` 增加 `CallsEnabled bool`，`New` 把它抄进 `server`。注册：

```go
mux.HandleFunc("GET /api/features", s.handleFeatures)
```

```go
func (s *server) handleFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"calls": s.callsEnabled})
}
```

使用项目里已有的 `writeJSON` 签名。如果现有函数不接收状态码，就按现有签名只传 body。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ ./cmd/superlocal/ ./internal/httpapi/ -count=1 -run 'TestFromArgs|TestBanner|TestFeaturesReflectCalls'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/superlocal/main.go cmd/superlocal/banner_test.go internal/httpapi/server.go internal/httpapi/features_test.go
git commit -m "$(cat <<'EOF'
feat(server): 默认使用 HTTPS 并公布通话开关

EOF
)"
```

---

### Task 3: 通话状态机

**Files:**
- Create: `internal/call/session.go`
- Test: `internal/call/session_test.go`

**Interfaces:**
- Consumes: 无
- Produces:

```go
type Turn struct {
    URLs       []string `json:"urls"`
    Username   string   `json:"username"`
    Credential string   `json:"credential"`
}
type Event struct {
    Type, CallID, ConversationID, Kind, FromUserID, FromNickname, Reason, SDP string
    Candidate json.RawMessage
    Turn      *Turn
}
type Delivery struct {
    UserIDs []string
    Event   Event
}
func New(now func() time.Time, id func(time.Time) string, turn func(userID string) Turn) *Session
func (s *Session) Invite(callerID, nickname, conversationID, kind string, memberIDs []string, peerOnline bool) []Delivery
func (s *Session) Accept(userID, callID string) []Delivery
func (s *Session) Reject(userID, callID string) []Delivery
func (s *Session) Hangup(userID, callID string) []Delivery
func (s *Session) Offer(userID, callID, sdp string) []Delivery
func (s *Session) Answer(userID, callID, sdp string) []Delivery
func (s *Session) Candidate(userID, callID string, candidate json.RawMessage) []Delivery
func (s *Session) Disconnect(userID string) []Delivery
func (s *Session) Sweep(now time.Time) []Delivery
```

JSON 字段名与规格表格一致：`callId`、`conversationId`、`kind`、`fromUserId`、`fromNickname`、`reason`、`sdp`、`candidate`、`turn`。

- [ ] **Step 1: Write the failing test**

```go
package call

import (
	"testing"
	"time"
)

func testSession(now time.Time) *Session {
	return New(func() time.Time { return now }, func(time.Time) string { return "call-1" }, func(string) Turn {
		return Turn{URLs: []string{"stun:127.0.0.1:8080"}, Username: "u", Credential: "p"}
	})
}

func TestInviteRejectsGroupSizedMemberList(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	out := s.Invite("a", "甲", "conv", "audio", []string{"a", "b", "c"}, true)
	if len(out) != 1 || out[0].Event.Reason != "forbidden" {
		t.Fatalf("%+v", out)
	}
}

func TestInviteOfflineBusyAndTimeout(t *testing.T) {
	now := time.Unix(100, 0)
	s := testSession(now)
	off := s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, false)
	if off[0].Event.Reason != "offline" {
		t.Fatalf("%+v", off)
	}
	ring := s.Invite("a", "甲", "conv", "video", []string{"a", "b"}, true)
	if ring[0].Event.Type != "call.ringing" || ring[1].Event.Type != "call.invite" || ring[1].Event.CallID != "call-1" {
		t.Fatalf("%+v", ring)
	}
	busy := s.Invite("c", "丙", "other", "audio", []string{"c", "a"}, true)
	if busy[0].Event.Reason != "busy" {
		t.Fatalf("%+v", busy)
	}
	timed := s.Sweep(now.Add(31 * time.Second))
	if len(timed) != 1 || timed[0].Event.Reason != "timeout" {
		t.Fatalf("%+v", timed)
	}
	again := s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	if again[0].Event.Type != "call.ringing" {
		t.Fatalf("not free: %+v", again)
	}
}

func TestRejectHangupAndDisconnect(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	rejected := s.Reject("b", "call-1")
	if rejected[0].Event.Reason != "reject" || rejected[0].UserIDs[0] != "a" {
		t.Fatalf("%+v", rejected)
	}
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	s.id = func(time.Time) string { return "call-2" }
	_ = s.Hangup("a", "call-1")
	// call-1 已结束。重新邀请会使用下一次 id。
	s.id = func(time.Time) string { return "call-2" }
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	gone := s.Disconnect("b")
	if gone[0].Event.Type != "call.hangup" || gone[0].UserIDs[0] != "a" {
		t.Fatalf("%+v", gone)
	}
}

func TestOfferWaitsForAccept(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	if len(s.Offer("a", "call-1", "offer")) != 0 {
		t.Fatal("offer before accept")
	}
	s.Accept("b", "call-1")
	offered := s.Offer("a", "call-1", "offer")
	if offered[0].Event.Type != "call.offer" || offered[0].UserIDs[0] != "b" {
		t.Fatalf("%+v", offered)
	}
	answered := s.Answer("b", "call-1", "answer")
	if answered[0].UserIDs[0] != "a" || answered[0].Event.SDP != "answer" {
		t.Fatalf("%+v", answered)
	}
}
```

把 `Session.id` 改成测试可替换会破坏封装。实现时不要导出可变字段。测试里第二次邀请前构造一个新的 `Session`，或者让 `New` 的 `id` 函数从切片里依次取 ID。测试用这个 id 函数：

```go
func seqIDs(ids ...string) func(time.Time) string {
	i := 0
	return func(time.Time) string {
		id := ids[i]
		if i < len(ids)-1 {
			i++
		}
		return id
	}
}
```

`TestRejectHangupAndDisconnect` 使用 `seqIDs("call-1", "call-2")`，不要给 `s.id` 赋值。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/call/ -count=1`

Expected: FAIL with `undefined: New`

- [ ] **Step 3: Write minimal implementation**

`Invite` 的规则：

- `kind` 不是 `audio` 或 `video`，或 `callerID` 不在 `memberIDs`，或 `len(memberIDs) != 2`：只给主叫回 `call.reject`，`reason` 是 `forbidden`，不占用用户。
- 对方不在线：`offline`。
- `byUser` 里已有主叫或对方：`busy`。
- 否则创建 `ringing` 通话，记录开始时间、主叫、被叫和会话。给主叫 `call.ringing`，给被叫 `call.invite`。两边都带 `turn`。被叫事件包含 `fromUserId` 和 `fromNickname`。

`Accept` 只有被叫可以对 `ringing` 通话调用。成功后状态改为 `accepted`，给主叫 `call.accept`，给被叫 `call.end`。

`Reject` 只有被叫可以调用。给主叫 `call.reject`，原因 `reject`，然后删除占用。

`Hangup` 通话双方都可以调用。给另一方 `call.hangup`，然后删除占用。

`Offer` 只在状态为 `accepted` 且发送者是主叫时转给被叫。`Answer` 只在此后且发送者是被叫时转给主叫。`Candidate` 在 `accepted` 之后由一方向另一方转发。不满足条件时返回空切片。

`Disconnect` 找到该用户的通话，效果与该用户 `Hangup` 相同。

`Sweep` 找出开始时间早于 `now-30s` 且仍是 `ringing` 的通话。给双方一条 `call.reject`，`reason` 是 `timeout`，用户列表包含主叫和被叫。然后删除占用。

所有方法先锁 `sync.Mutex`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/call/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/call/session.go internal/call/session_test.go
git commit -m "$(cat <<'EOF'
feat(call): 维护单聊通话状态

EOF
)"
```

---

### Task 4: WebSocket 信令

**Files:**
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/ws.go`
- Create: `internal/httpapi/call.go`
- Test: `internal/httpapi/call_test.go`

**Interfaces:**
- Consumes: `*call.Session` 的 `Invite`、`Accept`、`Reject`、`Hangup`、`Offer`、`Answer`、`Candidate`、`Disconnect`
- Produces: `Deps.Calls *call.Session`。WebSocket 识别 `call.invite`、`call.accept`、`call.reject`、`call.hangup`、`call.offer`、`call.answer`、`call.candidate`。

- [ ] **Step 1: Write the failing test**

仿照 `internal/httpapi/ws_test.go` 的登录和 `POST /api/conversations/direct`。三名用户甲、乙、丙。甲和乙建立单聊。甲通过 WebSocket 发送：

```json
{"type":"call.invite","conversationId":"<甲乙会话>","kind":"audio"}
```

乙要收到 `type == "call.invite"` 且 `callId` 非空。丙发送同一个 `conversationId` 的 `call.invite` 时，只收到自己的 `call.reject`，`reason` 为 `forbidden`，乙不会收到第二条邀请。

再测一方断开：甲乙进入 ringing 后关闭乙的 WebSocket，甲收到 `call.hangup`。

`Deps.Calls` 使用 `call.New`，TURN 函数返回固定的 `call.Turn`。`Now` 固定，便于断言。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi/ -count=1 -run TestCallSignaling`

Expected: FAIL，编译错误指出 `Deps` 没有 `Calls`，或运行时收到 `未知事件`。

- [ ] **Step 3: Write minimal implementation**

`wsIncoming` 增加 `CallID string json:"callId"`、`SDP string json:"sdp"`、`Candidate json.RawMessage json:"candidate"`。

`switch` 在 `default` 前增加这些 `call.*` 分支，都调用 `s.handleCall(conn, user, in)`。

`handleWS` 的 `defer` 在 `hub.Remove` 之后调用 `s.calls.Disconnect(user.ID)`，并把返回的 `Delivery` 用 `hub.Send` 发出。`s.calls` 为 nil 时跳过。

`handleCall`：

- `s.calls == nil` 时 `writeWSError(conn, "通话不可用")`。
- `call.invite` 读取 `SELECT type FROM conversations WHERE id = ?`。类型不是 `direct` 时按非成员处理，成员列表传 `nil`。类型是 `direct` 时用现有 `conversationMemberIDs`。对方是否在线看 `s.hub.OnlineIDs()`。昵称用 `user.Nickname`。
- 其余类型把 `in.CallID`、`in.SDP`、`in.Candidate` 交给对应方法。
- 每个 `Delivery` 用 `json.Marshal(delivery.Event)` 后 `s.hub.Send(delivery.UserIDs, payload)`。

`main.go` 创建 `call.New`，传入 `store.NewID` 和暂时返回空 `call.Turn` 的函数。Task 5 再填真实凭据。`-tls=false` 时 `Calls` 传 `nil`。

启动一个每秒调用 `Sweep(time.Now())` 的 goroutine，并把结果广播出去。进程退出时停止它。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/httpapi/ -count=1 -run 'TestCallSignaling|TestWSMessagePresenceAndKick'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/server.go internal/httpapi/ws.go internal/httpapi/call.go internal/httpapi/call_test.go cmd/superlocal/main.go
git commit -m "$(cat <<'EOF'
feat(call): 转发单聊通话信令

EOF
)"
```

---

### Task 5: TURN 凭据和 UDP 服务

**Files:**
- Create: `internal/turncred/cred.go`
- Test: `internal/turncred/cred_test.go`
- Create: `internal/turnserver/server.go`
- Test: `internal/turnserver/server_test.go`
- Modify: `cmd/superlocal/main.go`

**Interfaces:**
- Consumes: 监听端口号和局域网 IPv4
- Produces:

```go
func Mint(secret []byte, userID string, now time.Time) (username, credential string)
func Verify(secret []byte, username, credential string, now time.Time) bool
func URLs(port string, ips []string) []string
func Listen(addr string, relayIP net.IP, secret []byte) (*turn.Server, error)
```

`URLs` 对每个 IP 生成 `stun:<ip>:<port>` 和 `turn:<ip>:<port>?transport=udp`。没有 IP 时使用 `127.0.0.1`。

- [ ] **Step 1: Write the failing test**

```go
func TestMintExpiresAndMatches(t *testing.T) {
	secret := []byte("secret")
	now := time.Unix(1_700_000_000, 0)
	username, credential := Mint(secret, "user-1", now)
	if !Verify(secret, username, credential, now.Add(4*time.Minute)) {
		t.Fatal("fresh credential rejected")
	}
	if Verify(secret, username, credential, now.Add(6*time.Minute)) {
		t.Fatal("expired credential accepted")
	}
	if Verify(secret, username, "nope", now) {
		t.Fatal("wrong credential accepted")
	}
}
```

`turnserver` 测试占用一个 UDP `:0`，`Listen` 成功后 `Close()`。绑定失败由 `main` 用 `log.Fatalf` 退出，测试不覆盖进程退出。

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/turncred/ ./internal/turnserver/ -count=1`

Expected: FAIL with `undefined: Mint`

- [ ] **Step 3: Write minimal implementation**

用户名是 `expiryUnix:userID`，`expiryUnix` 是 `now+5min` 的秒。密码是 `HMAC-SHA1(secret, username)` 的标准 Base64。`Verify` 用 `hmac.Equal` 比较，并拒绝过期或没有冒号的用户名。

`Listen` 使用 `net.ListenPacket("udp", addr)`。`turn.NewServer` 的 Realm 固定为 `superlocal`。`AuthHandler` 只在 `Verify` 成功时返回 `turn.GenerateAuthKey(username, realm, credential)`。`RelayAddressGeneratorStatic` 的 `RelayAddress` 用传入的 `relayIP`，为空时用 `127.0.0.1`。`Address` 用 `0.0.0.0`。

执行 `go get github.com/pion/turn/v4`。

`main.go` 只在 `cfg.TLS` 时调用 `Listen`。地址是 `:<banner 使用的端口>`。中继 IP 取第一块局域网 IPv4，没有时用 `127.0.0.1`。`Listen` 返回错误时 `log.Fatalf("TURN 端口绑定失败: %v", err)`。`call.New` 的 TURN 函数返回 `call.Turn{URLs: turncred.URLs(port, lanIPv4()), Username: username, Credential: credential}`。进程退出前 `server.Close()`。

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/turncred/ ./internal/turnserver/ ./internal/call/ -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/turncred internal/turnserver cmd/superlocal/main.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(call): 在本机中转失败的音视频

EOF
)"
```

---

### Task 6: 聊天页通话界面

**Files:**
- Create: `web/src/callState.ts`
- Test: `web/src/callState.test.ts`
- Create: `web/src/call.ts`
- Modify: `web/src/stores/chat.ts`
- Modify: `web/src/views/Chat.vue`
- Modify: `web/src/api.ts`

**Interfaces:**
- Consumes: `GET /api/features` 的 `{ calls: boolean }`，WebSocket 上 Task 4 的事件
- Produces: `reduceCall(state, event) -> CallView`。`startCall(kind)`、`acceptCall()`、`rejectCall()`、`hangupCall()`。

`CallView` 是：

```ts
export type CallView = {
  phase: 'idle' | 'outgoing' | 'incoming' | 'connecting' | 'active' | 'failed'
  callId: string
  conversationId: string
  kind: 'audio' | 'video'
  peerName: string
  reason: string
  muted: boolean
  cameraOn: boolean
}
```

空闲初始值的 `phase` 是 `idle`，其余字符串为空，`muted` 为 false，`cameraOn` 为 true。

- [ ] **Step 1: Write the failing test**

`web/src/callState.test.ts` 用 `node:test` 和 `node:assert/strict`。覆盖：

- `call.invite` 把 `idle` 变成 `incoming`，并保存 `callId`、`peerName`、`kind`。
- `incoming` 收到同一 `callId` 的 `call.end` 或 `call.reject` 后回到 `idle`。
- `outgoing` 收到 `call.accept` 后变成 `connecting`。
- `connecting` 收到 `call.answer` 后变成 `active`。
- `outgoing` 收到 `reason: "busy"` 后变成 `failed`。
- `active` 收到 `call.hangup` 后回到 `idle`。
- `connecting` 收到 `call.end` 保持 `connecting`。

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/usr/local/opt/node@24/bin:$PATH"; node --experimental-strip-types --test web/src/callState.test.ts`

Expected: FAIL，找不到模块。

- [ ] **Step 3: Write minimal implementation**

`reduceCall` 按上面的转移实现，未知事件返回原状态。

`call.ts` 持有一个 `RTCPeerConnection` 和本地 `MediaStream`。`iceServers` 来自信令里的 `turn`。创建连接时不设置 `iceTransportPolicy`，让主机候选优先，中继候选作为后备。

`startCall`：`window.isSecureContext` 为 false 或 `callsEnabled` 为 false 时直接返回。`getUserMedia({ audio: true, video: kind === 'video' })` 失败时把阶段设为 `failed`，`reason` 为 `无法使用麦克风或摄像头`，不发送 `call.invite`。成功后发送 `{ type: 'call.invite', conversationId, kind }`，阶段为 `outgoing`。

收到 `call.accept` 后才 `createOffer` 并发送 `call.offer`。收到 `call.offer` 后 `setRemoteDescription`、`createAnswer`、发送 `call.answer`。收到 `call.answer` 后 `setRemoteDescription`。`onicecandidate` 发送 `call.candidate`。`ontrack` 把远端流交给回调。`onconnectionstatechange` 在 `failed` 时把阶段设为 `failed`，`reason` 为 `连接失败`，并发送 `call.hangup`。

`acceptCall` 按 kind 获取设备，发送 `call.accept`，阶段改为 `connecting`。`rejectCall` 发送 `call.reject` 并回到 `idle`。`hangupCall` 发送 `call.hangup`，停止本地轨道，关闭连接，回到 `idle`。静音切换本地音频轨道的 `enabled`。关闭摄像头时关闭视频轨道的 `enabled`，远端画面位置改显示昵称，不保留最后一帧。可以用 `v-if="cameraOn"` 卸掉本地 `<video>`。

`api.ts` 增加 `export function features() { return http.get<{ calls: boolean }>('/api/features') }`，沿用现有的 axios 实例。

`chat.ts` 在启动时读取 features，保存 `callsEnabled`。`handleSocketData` 对 `call.` 开头的事件调用 `call.ts` 导出的 `onSignal(event)`。`sendRaw(payload)` 在 socket 打开时 `socket.send(JSON.stringify(payload))`。

`Chat.vue` 的 `.head-actions` 里，群按钮之前加入：

```vue
<button
  v-if="canCall"
  type="button"
  class="head-btn"
  @click="startCall('audio')"
>语音</button>
<button
  v-if="canCall"
  type="button"
  class="head-btn"
  @click="startCall('video')"
>视频</button>
```

`canCall` 是 `store.callsEnabled && window.isSecureContext && store.active?.type === 'direct'`。

来电和通话浮层放在 `.thread` 内、消息列表之前：

```vue
<div v-if="call.phase !== 'idle'" class="call-layer">
  <p class="call-name">{{ call.peerName || store.active?.title }}</p>
  <p v-if="call.phase === 'failed'" class="call-reason">{{ call.reason === 'busy' ? '对方忙线' : call.reason === 'timeout' ? '对方未接听' : call.reason === 'offline' ? '对方离线' : call.reason }}</p>
  <video v-if="call.kind === 'video' && call.phase === 'active' && remoteStream" ref="remoteVideo" autoplay playsinline></video>
  <video v-if="call.kind === 'video' && call.cameraOn && localStream" ref="localVideo" autoplay playsinline muted></video>
  <div class="call-actions">
    <button v-if="call.phase === 'incoming'" type="button" class="head-btn" @click="acceptCall">接听</button>
    <button v-if="call.phase === 'incoming'" type="button" class="head-btn danger" @click="rejectCall">拒绝</button>
    <button v-if="call.phase === 'active'" type="button" class="head-btn" @click="toggleMute">{{ call.muted ? '取消静音' : '静音' }}</button>
    <button v-if="call.phase === 'active' && call.kind === 'video'" type="button" class="head-btn" @click="toggleCamera">{{ call.cameraOn ? '关闭摄像头' : '打开摄像头' }}</button>
    <button v-if="call.phase === 'outgoing' || call.phase === 'connecting' || call.phase === 'active'" type="button" class="head-btn danger" @click="hangupCall">挂断</button>
  </div>
</div>
```

样式：`.call-layer` 白底、1px `#E6E8EB` 边框、圆角 8px，盖住聊天区。主按钮文字 `#0B6BCB`，说明文字 `#646A73`，标题 `#1F2329`。窄屏下浮层仍在 `.thread` 里。

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/usr/local/opt/node@24/bin:$PATH"; node --experimental-strip-types --test web/src/callState.test.ts && cd web && npm run build`

Expected: 测试 PASS，`vue-tsc` 和 Vite 构建成功。

- [ ] **Step 5: Commit**

```bash
git add web/src/callState.ts web/src/callState.test.ts web/src/call.ts web/src/stores/chat.ts web/src/views/Chat.vue web/src/api.ts
git commit -m "$(cat <<'EOF'
feat(web): 单聊可以发起语音和视频

EOF
)"
```

不要提交 `internal/web/dist/index.html`。构建产生的哈希资源仍由 `.gitignore` 忽略。

---

### Task 7: 桌面程序改用 HTTPS

**Files:**
- Modify: `desktop/src/banner.js`
- Modify: `desktop/test/banner.test.js`
- Modify: `desktop/src/main.js`

**Interfaces:**
- Consumes: 服务标准输出里的 `https://` 地址
- Produces: `parseBanner` 同时接受 `http://` 和 `https://`。`lanUrls` 生成 `https://`。`probeHttp` 用 `node:https` 访问 `127.0.0.1`，`rejectUnauthorized: false`。`openLocal` 打开 `https://127.0.0.1:<端口>/`。

- [ ] **Step 1: Write the failing test**

把 `desktop/test/banner.test.js` 的样例改成 `https://`。追加一个断言：只有旧的 `http://` 行时 `parseBanner` 仍能解析，避免半截日志被丢掉。`lanUrls` 不在单元测试里绑定网卡；只断言 `parseBanner` 和 `urlsFromBanner`。

- [ ] **Step 2: Run test to verify it fails**

Run: `export PATH="/usr/local/opt/node@24/bin:$PATH"; cd desktop && npm test`

Expected: FAIL，`parseBanner` 得到空数组。

- [ ] **Step 3: Write minimal implementation**

`parseBanner` 的判断从 `startsWith("http://")` 改成同时接受 `https://`。`lanUrls` 里的两个地址改成 `https://`。`isShareable` 的探测地址也改成 `https://`，它只检查主机名。

`main.js` 的 `probeHttp`：

```js
import https from "node:https";

function probeHttp(port) {
  return new Promise((resolve) => {
    const req = https.get(
      { hostname: "127.0.0.1", port, path: "/", rejectUnauthorized: false, timeout: 1000 },
      (res) => {
        res.resume();
        resolve(true);
      },
    );
    req.on("error", () => resolve(false));
    req.setTimeout(1000, () => {
      req.destroy();
      resolve(false);
    });
  });
}
```

`stdoutText.includes("http://")` 改成 `includes("https://")`。`openLocal` 的 `base` 改成 `` `https://127.0.0.1:${state.port}` ``。

- [ ] **Step 4: Run test to verify it passes**

Run: `export PATH="/usr/local/opt/node@24/bin:$PATH"; cd desktop && npm test`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/src/banner.js desktop/test/banner.test.js desktop/src/main.js
git commit -m "$(cat <<'EOF'
feat(desktop): 用 HTTPS 检查并打开本机服务

EOF
)"
```

---

## Spec coverage

- HTTPS、证书内容、权限和重新生成：Task 1、Task 2。
- `-tls=false`、功能开关和隐藏按钮：Task 2、Task 6。
- 单聊限制、忙线、离线、超时、挂断、断开和 offer 顺序：Task 3、Task 4、Task 6。
- TURN 凭据、UDP 端口和中转：Task 5。
- 来电层、静音、摄像头和失败文案：Task 6。
- 桌面地址和自签健康检查：Task 7。

## Placeholder scan

计划里的测试和接口都写出了具体名字。Task 4 的 WebSocket 测试步骤指定了请求体和断言，实现时按 `ws_test.go` 的 `loginClient` 和 `websocket.Dial` 写完整测试函数，不另起一套客户端。

## Type consistency

`call.Turn`、`call.Delivery`、`call.Session` 的方法名在 Task 3、Task 4、Task 5 中一致。前端 `CallView.phase` 和 `reduceCall` 只在 Task 6 使用。`GET /api/features` 的 `calls` 字段在 Task 2 和 Task 6 一致。

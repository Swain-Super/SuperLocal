# 局域网固定名字 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 别人用 `http://superlocal.local:<端口>/` 或扫二维码进入现有聊天页；在 HTTP 页面点语音、视频或共享屏幕时，只提示改用 HTTPS。

**Architecture:** `internal/discover` 选择名字并收发 mDNS A 记录。`cmd/superlocal` 在启动输出里写 `名字 <host>`，并把该名字放进自签证书。桌面窗口只解析这一行、画出二维码。聊天页在 HTTP 上仍显示三个按钮，但点击只给出 HTTPS 地址。

**Tech Stack:** Go 1.22、标准库 `net`（UDP 组播 224.0.0.251:5353）、现有 `crypto/x509`、Vue 3。不新增 Go 模块，不给聊天页加依赖。`CGO_ENABLED=0`。

## Global Constraints

- 不新增命令行参数，不占用 80 端口。`-tls=false` 时同样广播名字。
- 只登记主机名的 A 记录，指向可分享的局域网 IPv4。不要回环、不要 `169.254.`、不要 IPv6。只广播服务类型而不登记主机名不算完成。
- 名字顺序是 `superlocal.local`，然后 `superlocal-2.local`，直到 `superlocal-9.local`。A 记录为空，或全部属于本机这些 IPv4，才用这个名字。九个都被别人占用时不广播。进程活着时不改名字。
- 没有可分享 IPv4 时不广播，桌面不显示名字和二维码。
- 每 15 秒核对 IPv4，集合变了就更新 A 记录。服务停止时撤销广播。5353 绑不上时服务仍启动，不打印名字行。
- `cfg.TLS` 为真时，证书 DNS 名包含 `localhost` 和选中的名字。名字不在证书里就重新生成。`-tls=false` 不生成证书。
- 启动输出在地址列表之后、账号之前写 `名字 superlocal.local`。这一行不是 URL，不进入按 IP 排列的地址列表。
- 桌面局域网名字的文案是 `http://<名字>:<端口>/`。二维码内容是地址列表里第一条局域网聊天地址中的 `http://`。点「刷新」只重画窗口。「打开聊天」和「打开后台」仍是 `https://127.0.0.1:<端口>`。
- HTTP 且 `calls` 为真时，单聊显示三个按钮。点击不发 `call.*` 或 `screen.*`。说明原文是「语音、视频和共享屏幕需要用 HTTPS 打开」，地址只把当前页面的协议换成 `https:`，主机和端口不变。`calls` 为假或群聊时不显示按钮。HTTP 页面收到的 `call.*`、`screen.*` 仍忽略，服务仍拒绝非 TLS 上的这两类信令。

## File Structure

| 路径 | 职责 |
| --- | --- |
| `internal/discover/name.go` | 可分享 IPv4、候选名字、按已有 A 记录选名字 |
| `internal/discover/name_test.go` | 上述纯函数 |
| `internal/discover/mdns.go` | 组查询和 A 记录应答，不压缩自己发出的名字 |
| `internal/discover/mdns_test.go` | 报文往返；占用探测用注入的查询函数 |
| `internal/discover/announce.go` | 绑定 5353、应答查询、15 秒复核由调用方驱动 |
| `internal/tlscert/cert.go` | 非 IP 主机名写入证书 DNS 名 |
| `cmd/superlocal/main.go` | 选名字、发证书、打印、停止时撤销 |
| `desktop/src/banner.js` | 从启动输出取出名字 |
| `desktop/src/qr.js` | 版本 3、纠错 L、字节模式的二维码矩阵 |
| `desktop/renderer/App.vue` | 名字和二维码 |
| `web/src/httpsPage.ts` | 把当前页换成 HTTPS 地址 |
| `web/src/views/Chat.vue` | HTTP 上的三个按钮只出说明 |
| `README.md` | 用固定名字和二维码进入 |

### Task 1: 选择名字

**Files:**
- Create: `internal/discover/name.go`
- Test: `internal/discover/name_test.go`

**Interfaces:**
- Produces: `func ShareableIPv4(ips []net.IP) []net.IP`，`func Candidates() []string`，`func PickName(candidates []string, ours []net.IP, found map[string][]net.IP) string`。`found` 的键是不带点结尾的主机名。没有记录时键不存在或切片为空。返回空字符串表示九个名字都不能用。

- [ ] **Step 1: Write the failing test**

```go
func TestShareableIPv4(t *testing.T) {
	got := ShareableIPv4([]net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("169.254.1.1"),
		net.ParseIP("fe80::1"),
		net.ParseIP("192.168.1.8"),
		net.ParseIP("192.168.1.8"),
		net.ParseIP("10.0.0.2"),
	})
	if len(got) != 2 || got[0].String() != "192.168.1.8" || got[1].String() != "10.0.0.2" {
		t.Fatalf("%v", got)
	}
}

func TestPickName(t *testing.T) {
	ours := []net.IP{net.ParseIP("192.168.1.8")}
	names := Candidates()
	if names[0] != "superlocal.local" || names[1] != "superlocal-2.local" || names[len(names)-1] != "superlocal-9.local" || len(names) != 9 {
		t.Fatalf("%v", names)
	}
	if got := PickName(names, ours, nil); got != "superlocal.local" {
		t.Fatalf("empty: %s", got)
	}
	own := map[string][]net.IP{"superlocal.local": {net.ParseIP("192.168.1.8")}}
	if got := PickName(names, ours, own); got != "superlocal.local" {
		t.Fatalf("ours: %s", got)
	}
	taken := map[string][]net.IP{"superlocal.local": {net.ParseIP("192.168.1.9")}}
	if got := PickName(names, ours, taken); got != "superlocal-2.local" {
		t.Fatalf("next: %s", got)
	}
	all := map[string][]net.IP{}
	for _, name := range names {
		all[name] = []net.IP{net.ParseIP("10.1.1.1")}
	}
	if got := PickName(names, ours, all); got != "" {
		t.Fatalf("full: %s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/discover/ -count=1`
Expected: FAIL，包不存在。

- [ ] **Step 3: Write minimal implementation**

`ShareableIPv4` 跳过 `To4() == nil`、`IsLoopback`、`IsLinkLocalUnicast`，并按输入顺序去重。`Candidates` 先放 `superlocal.local`，再放 `superlocal-2.local` 到 `superlocal-9.local`。`PickName` 按候选顺序看 `found[name]`：长度为 0 就返回；每条都 `To4()` 后属于 `ours` 也返回；否则看下一个。比较用 `ip.String()`。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/discover/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/discover/name.go internal/discover/name_test.go
git commit -m "$(cat <<'EOF'
feat(discover): 按占用情况选择局域网名字

EOF
)"
```

### Task 2: 收发 A 记录

**Files:**
- Create: `internal/discover/mdns.go`
- Create: `internal/discover/announce.go`
- Test: `internal/discover/mdns_test.go`

**Interfaces:**
- Consumes: `ShareableIPv4`、`PickName`、`Candidates`
- Produces: `func QueryA(name string) []byte`，`func ParseA(packet []byte, name string) []net.IP`，`func ResponseA(name string, ips []net.IP) []byte`，`func Choose(ours []net.IP, lookup func(name string) []net.IP) string`。`Announce` 结构有 `Close()` 和 `Update(ips []net.IP)`。`func Serve(name string, ips []net.IP, conn net.PacketConn) *Announce`。组播地址是 `224.0.0.251:5353`。自己发出的问题名和答案名都不使用压缩指针。

- [ ] **Step 1: Write the failing test**

```go
func TestQueryAndParseA(t *testing.T) {
	q := QueryA("superlocal.local")
	if len(q) < 12 || q[2] != 0 || q[3] != 0 || q[5] != 1 {
		t.Fatalf("query header %x", q[:6])
	}
	packet := ResponseA("superlocal.local", []net.IP{net.ParseIP("192.168.1.8"), net.ParseIP("10.0.0.2")})
	got := ParseA(packet, "superlocal.local")
	if len(got) != 2 || got[0].String() != "192.168.1.8" || got[1].String() != "10.0.0.2" {
		t.Fatalf("%v", got)
	}
	if len(ParseA(packet, "other.local")) != 0 {
		t.Fatal("other name matched")
	}
}

func TestParseCompressedA(t *testing.T) {
	// 答案名用指针 0xC00C 指回问题里的 superlocal.local，确认解析能跟指针。
	var raw []byte
	raw = append(raw, 0, 1, 0x84, 0, 0, 1, 0, 1, 0, 0, 0, 0)
	raw = append(raw, 11)
	raw = append(raw, []byte("superlocal")...)
	raw = append(raw, 5)
	raw = append(raw, []byte("local")...)
	raw = append(raw, 0, 0, 1, 0, 1)
	raw = append(raw, 0xC0, 0x0C, 0, 1, 0x80, 1, 0, 0, 0, 120, 0, 4, 192, 168, 1, 8)
	got := ParseA(raw, "superlocal.local")
	if len(got) != 1 || got[0].String() != "192.168.1.8" {
		t.Fatalf("%v", got)
	}
}

func TestChooseUsesLookupOrder(t *testing.T) {
	ours := []net.IP{net.ParseIP("192.168.1.8")}
	got := Choose(ours, func(name string) []net.IP {
		if name == "superlocal.local" {
			return []net.IP{net.ParseIP("10.9.9.9")}
		}
		return nil
	})
	if got != "superlocal-2.local" {
		t.Fatalf("%s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/discover/ -count=1 -run 'TestQueryAndParseA|TestParseCompressedA|TestChooseUsesLookupOrder'`
Expected: FAIL，`QueryA` 未定义。

- [ ] **Step 3: Write minimal implementation**

`QueryA` 写出 12 字节头：ID 固定 0，标志 0，QDCOUNT 1。问题名按点分段，每段一字节长度加原文，以 0 结束。QTYPE 为 1，QCLASS 为 1。

`ResponseA` 标志为 `0x8400`，QDCOUNT 1，ANCOUNT 为 IPv4 条数。问题与 `QueryA` 相同。每条答案的名字再次写完整标签，TYPE 1，CLASS `0x8001`，TTL 120，RDLENGTH 4，RDATA 为 4 字节 IPv4。

`ParseA` 只看答案区。名字以 `0xC0` 开头时，指针是后 14 位，跳到该偏移继续读，指针最多跟 8 次。标签拼成小写、用点连接、去掉结尾点。TYPE 不是 1 或 RDLENGTH 不是 4 的答案跳过。只收集名字与参数相同的地址。

`Choose` 对 `Candidates()` 调用 `lookup`，把结果放进 map 后交给 `PickName`。

`Serve` 在循环里 `ReadFrom`。报文是对 `name` 的 A 或 ANY（TYPE 255）查询时，`WriteTo` `ResponseA` 到 `224.0.0.251:5353`。`Update` 替换 IP 并立刻再发一次应答。`Close` 关闭连接使循环退出。`Announce(name string, ips []net.IP) (*Announce, error)` 用 `net.ListenConfig` 的 `Control` 设置 `SO_REUSEADDR` 和 `SO_REUSEPORT`，`ListenPacket` 到 `0.0.0.0:5353`，加入组播 `224.0.0.251`，然后 `Serve`。绑不上就返回错误，调用方不因此退出进程。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/discover/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/discover/mdns.go internal/discover/mdns_test.go internal/discover/announce.go
git commit -m "$(cat <<'EOF'
feat(discover): 收发局域网主机名的 A 记录

EOF
)"
```

### Task 3: 接到服务进程

**Files:**
- Modify: `internal/tlscert/cert.go`
- Modify: `internal/tlscert/cert_test.go`
- Modify: `cmd/superlocal/main.go`
- Modify: `cmd/superlocal/banner_test.go`
- Test: `internal/tlscert/cert_test.go`

**Interfaces:**
- Consumes: `discover.ShareableIPv4`、`discover.Choose`、`discover.Announce`、`discover.ParseA`。查询函数对每个候选名把 `QueryA` 发到 `224.0.0.251:5353`，等 300 毫秒，用 `ParseA` 收集。
- Produces: `banner` 增加参数 `lanName string`。有名字时在账号行之前写 `名字 superlocal.local\n`。

- [ ] **Step 1: Write the failing test**

在 `cert_test.go` 增加：`LoadOrCreate(dir, []string{"192.168.1.8", "superlocal.local"}, now)` 之后，`leaf.VerifyHostname("superlocal.local")` 和 `leaf.VerifyHostname("192.168.1.8")` 都成功。再调用一次，证书字节不变。

把 `TestBanner` 的期望字符串改成在 `http://192.168.1.8:8080/admin` 之后、`账号 admin` 之前插入 `名字 superlocal.local`。调用改为 `banner(":8080", "admin", "secret", []string{"192.168.1.8"}, true, "superlocal.local")`。`TestBannerHTTPOnly` 传入空名字，期望里没有 `名字`。

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/tlscert/ ./cmd/superlocal/ -count=1`
Expected: FAIL，`banner` 参数数量不对，或证书没有这个 DNS 名。

- [ ] **Step 3: Write minimal implementation**

`hostSet` 仍把非空字符串放进集合。生成证书时 `DNSNames` 改为 `want` 里 `net.ParseIP` 失败的那些名字，排序后写入。IP 仍只进 `IPAddresses`。`covers` 已经用 `VerifyHostname`，DNS 名不在证书里会重新生成。

`main` 在生成证书之前：

```go
ips := discover.ShareableIPv4(ipsFromStrings(hosts))
lanName := ""
var announced *discover.Announce
if len(ips) > 0 {
	lanName = discover.Choose(ips, lookupA)
	if lanName != "" {
		announced, _ = discover.Announce(lanName, ips)
	}
}
```

`cfg.TLS` 为真时，把 `lanName` 追加进传给 `LoadOrCreate` 的字符串切片。`banner(..., lanName)`。另起 goroutine，每 15 秒把最新的 `ShareableIPv4` 交给 `announced.Update`；名字为空或 `Announce` 失败则不启动它。`http.Serve` 返回后调用 `announced.Close()`。`-tls=false` 也走选择和广播，只是不调用 `LoadOrCreate`。

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/tlscert/ ./internal/discover/ ./cmd/superlocal/ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tlscert/cert.go internal/tlscert/cert_test.go cmd/superlocal/main.go cmd/superlocal/banner_test.go
git commit -m "$(cat <<'EOF'
feat(discover): 启动时广播局域网名字

EOF
)"
```

### Task 4: 桌面显示名字和二维码

**Files:**
- Modify: `desktop/src/banner.js`
- Modify: `desktop/test/banner.test.js`
- Create: `desktop/src/qr.js`
- Create: `desktop/test/qr.test.js`
- Modify: `desktop/src/main.js`
- Modify: `desktop/renderer/App.vue`

**Interfaces:**
- Consumes: 启动输出里的 `名字 superlocal.local`
- Produces: `parseLanName(text) -> string`，没有则 `""`。`qrModules(text) -> number[][]`，边长 29。`state.lanName` 为 `http://<名字>:<端口>/` 或空字符串。

- [ ] **Step 1: Write the failing test**

```js
test("从启动输出取出局域网名字", () => {
  const text = "http://192.168.1.8:8080/\n名字 superlocal.local\n账号 admin\n";
  assert.equal(parseLanName(text), "superlocal.local");
  assert.equal(parseLanName("名字 \n"), "");
  assert.deepEqual(urlsFromBanner(text, 8080).chatUrls, ["http://192.168.1.8:8080/"]);
});

test("二维码三个角有定位图案", () => {
  const grid = qrModules("http://192.168.1.8:8080/");
  assert.equal(grid.length, 29);
  assert.equal(grid[0][0], 1);
  assert.equal(grid[0][6], 1);
  assert.equal(grid[6][0], 1);
  assert.equal(grid[0][22], 1);
  assert.equal(grid[22][0], 1);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --test desktop/test/banner.test.js desktop/test/qr.test.js`
Expected: FAIL，`parseLanName` 或 `qrModules` 未导出。

- [ ] **Step 3: Write minimal implementation**

`parseLanName` 找以 `名字 ` 开头的行，余下部分必须以 `.local` 结尾且不含空格。`urlsFromBanner` 继续只收 `http://` 和 `https://`。

`qrModules` 生成版本 3、纠错级别 L、字节模式，边长 29。定位图案在 `(0,0)`、`(22,0)`、`(0,22)`，图案是 7×7 黑框、白圈、3×3 黑心。时序图案在第 6 行和第 6 列，深色模块落在偶数坐标。数据从右下角按两列一组向上再向下填入，跳过功能图案。掩码只用模式 0：`(row + col) % 2 == 0` 的数据模块取反。格式信息用 ECC-L、掩码 0 的 15 位 `111011111000100`，沿定位图案写入，最低位在前。

`main.js` 的 `state` 增加 `lanName: ""`。`runStart` 读到启动输出后 `state.lanName = formatLanName(parseLanName(stdoutText), port)`。`formatLanName` 在名字为空时返回 `""`，否则返回 `http://<名字>:<端口>/`。接管旧进程时没有这行输出，`lanName` 保持空，二维码仍由地址列表产生。

`App.vue` 在「访问地址」里，有局域网 `http://` 时显示二维码：取 `chatUrls` 里第一条以 `http://` 开头的地址，用 `qrModules` 画 29×29 的小方块。`lanName` 非空时在二维码下显示该文案和一个「复制」按钮，复制的是这个 `http://名字/`。没有局域网地址时仍只显示「当前没有局域网地址」。

- [ ] **Step 4: Run test to verify it passes**

Run: `node --test desktop/test/banner.test.js desktop/test/qr.test.js desktop/test/*.js`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add desktop/src/banner.js desktop/test/banner.test.js desktop/src/qr.js desktop/test/qr.test.js desktop/src/main.js desktop/renderer/App.vue
git commit -m "$(cat <<'EOF'
feat(desktop): 显示局域网名字和二维码

EOF
)"
```

### Task 5: HTTP 页面提示改用 HTTPS

**Files:**
- Modify: `web/src/httpsPage.ts`
- Modify: `web/src/httpsPage.test.ts`
- Modify: `web/src/views/Chat.vue`
- Modify: `README.md`

**Interfaces:**
- Consumes: `isHttpsPage`
- Produces: `httpsPageURL(pageURL: string): string`。结果协议是 `https:`，路径是 `/`，查询和片段为空。

- [ ] **Step 1: Write the failing test**

```ts
test("HTTP 地址换成同一主机的 HTTPS", () => {
  assert.equal(httpsPageURL("http://superlocal.local:18080/chat"), "https://superlocal.local:18080/");
  assert.equal(httpsPageURL("http://192.168.1.8:8080/"), "https://192.168.1.8:8080/");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `node --experimental-strip-types --test web/src/httpsPage.test.ts`
Expected: FAIL，`httpsPageURL` 未导出。

- [ ] **Step 3: Write minimal implementation**

`httpsPageURL` 用 `URL` 改 `protocol`、`pathname`、`search`、`hash` 后 `toString()`。

`Chat.vue` 增加 `canOfferCall`：`store.callsEnabled && store.active?.type === 'direct'`。三个按钮的 `v-if` 改为 `canOfferCall`。`onStartCall` 和 `onShareScreen` 在 `!isHttpsPage(location.protocol)` 时设置说明，不调用 `startCall` 或 `startScreen`。说明包含固定句子「语音、视频和共享屏幕需要用 HTTPS 打开」和 `httpsPageURL(location.href)`，并有复制按钮。`https://` 上的点击路径保持现在的发起逻辑。群聊和 `callsEnabled === false` 仍然没有这三个按钮。

`README.md` 的使用说明加两句：同一局域网可打开 `http://superlocal.local:端口/`；桌面窗口的二维码是局域网 IP 的 `http://`。点语音、视频或共享屏幕时如果当前是 HTTP，页面会给出对应的 `https://` 地址。

- [ ] **Step 4: Run test to verify it passes**

Run: `node --experimental-strip-types --test web/src/httpsPage.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/httpsPage.ts web/src/httpsPage.test.ts web/src/views/Chat.vue README.md
git commit -m "$(cat <<'EOF'
feat(web): HTTP 页面提示改用 HTTPS

EOF
)"
```

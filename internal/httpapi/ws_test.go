package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"superlocal/internal/auth"
	"superlocal/internal/hub"
	"superlocal/internal/httpapi"
)

func TestWSMessagePresenceAndKick(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := &now
	db := openHTTPDB(t)
	dir := t.TempDir()
	h := hub.New()
	limiter := auth.NewLimiter()
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       h,
		DataDir:   dir,
		MaxUpload: 1024 * 1024,
		Limiter:   limiter,
		Now:       func() time.Time { return *clock },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "ws-key-jia", "甲")
	yi := loginClient(t, srv.URL, "ws-key-yi", "乙")
	bing := loginClient(t, srv.URL, "ws-key-bing", "丙")

	jiaMe := getMe(t, jia.client, srv.URL)
	yiMe := getMe(t, yi.client, srv.URL)
	jiaID := jiaMe["id"].(string)
	yiID := yiMe["id"].(string)

	// 打开一对一会话。
	openBody, _ := json.Marshal(map[string]string{"userId": yiID})
	openRes, err := jia.client.Post(srv.URL+"/api/conversations/direct", "application/json", bytes.NewReader(openBody))
	if err != nil {
		t.Fatal(err)
	}
	openRaw, _ := io.ReadAll(openRes.Body)
	openRes.Body.Close()
	if openRes.StatusCode != 200 {
		t.Fatalf("open direct status=%d body=%s", openRes.StatusCode, openRaw)
	}
	var openOut map[string]string
	if err := json.Unmarshal(openRaw, &openOut); err != nil {
		t.Fatal(err)
	}
	convID := openOut["id"]
	if convID == "" {
		t.Fatal("empty conversation id")
	}

	// 丙先连上，用于收 presence。
	bingConn := dialWS(t, srv.URL, bing.client)
	t.Cleanup(func() { _ = bingConn.Close(websocket.StatusNormalClosure, "") })

	jiaConn := dialWS(t, srv.URL, jia.client)
	t.Cleanup(func() { _ = jiaConn.Close(websocket.StatusNormalClosure, "") })

	// 丙应收到甲上线的 presence。
	pres := readWSPresence(t, bingConn, jiaID, true, 3*time.Second)

	yiConn := dialWS(t, srv.URL, yi.client)
	t.Cleanup(func() { _ = yiConn.Close(websocket.StatusNormalClosure, "") })
	// 排空甲/丙侧的乙上线 presence，避免干扰后续断言。
	_ = readWSPresence(t, jiaConn, yiID, true, 2*time.Second)
	_ = readWSPresence(t, bingConn, yiID, true, 2*time.Second)
	_ = pres

	sendPayload, _ := json.Marshal(map[string]any{
		"type":           "message.send",
		"conversationId": convID,
		"kind":           "text",
		"text":           "你好",
	})
	writeWS(t, jiaConn, sendPayload)

	yiNew := readWSType(t, yiConn, "message.new", 3*time.Second)
	jiaNew := readWSType(t, jiaConn, "message.new", 3*time.Second)

	for name, ev := range map[string]map[string]any{"yi": yiNew, "jia": jiaNew} {
		msg, ok := ev["message"].(map[string]any)
		if !ok {
			t.Fatalf("%s message.new missing message: %#v", name, ev)
		}
		raw, _ := json.Marshal(msg)
		if strings.Contains(string(raw), `"key"`) || strings.Contains(string(raw), "ws-key-") {
			t.Fatalf("%s message.new must not contain key: %s", name, raw)
		}
		if msg["senderNickname"] != "甲" {
			t.Fatalf("%s senderNickname = %#v, want 甲", name, msg["senderNickname"])
		}
		if msg["senderId"] != jiaID {
			t.Fatalf("%s senderId = %#v, want %s", name, msg["senderId"], jiaID)
		}
		if msg["kind"] != "text" || msg["body"] != "你好" {
			t.Fatalf("%s body/kind = %#v", name, msg)
		}
		if msg["conversationId"] != convID {
			t.Fatalf("%s conversationId = %#v", name, msg["conversationId"])
		}
	}

	var msgCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, convID).Scan(&msgCount); err != nil {
		t.Fatal(err)
	}
	if msgCount != 1 {
		t.Fatalf("messages after send = %d, want 1", msgCount)
	}

	// 无发言权：只向发送方回 error，消息数不增加。
	if err := db.UpdateUserAdmin(jiaID, false, false, true, true); err != nil {
		t.Fatal(err)
	}
	writeWS(t, jiaConn, sendPayload)
	errEv := readWSType(t, jiaConn, "error", 3*time.Second)
	if errEv["error"] == nil || errEv["error"] == "" {
		t.Fatalf("error event missing error: %#v", errEv)
	}
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, convID).Scan(&msgCount); err != nil {
		t.Fatal(err)
	}
	if msgCount != 1 {
		t.Fatalf("messages after muted send = %d, want 1", msgCount)
	}
	// 恢复发言权，便于后续清理。
	if err := db.UpdateUserAdmin(jiaID, false, true, true, true); err != nil {
		t.Fatal(err)
	}

	// 管理员封禁甲 → 已连接套接字收到 session.revoked。
	username, password, err := auth.EnsureAdmin(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	adminJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	adminClient := &http.Client{Jar: adminJar}
	adminPayload, _ := json.Marshal(map[string]string{"username": username, "password": password})
	adminLogin, err := adminClient.Post(srv.URL+"/api/admin/session", "application/json", bytes.NewReader(adminPayload))
	if err != nil {
		t.Fatal(err)
	}
	adminBody, _ := io.ReadAll(adminLogin.Body)
	adminLogin.Body.Close()
	if adminLogin.StatusCode != 200 {
		t.Fatalf("admin login status=%d body=%s", adminLogin.StatusCode, adminBody)
	}
	patchPayload := `{"banned":true,"canSendMessage":true,"canSendFile":true,"canCreateGroup":true}`
	patchReq, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/admin/users/"+jiaID, strings.NewReader(patchPayload))
	if err != nil {
		t.Fatal(err)
	}
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes, err := adminClient.Do(patchReq)
	if err != nil {
		t.Fatal(err)
	}
	patchBody, _ := io.ReadAll(patchRes.Body)
	patchRes.Body.Close()
	if patchRes.StatusCode != 200 {
		t.Fatalf("ban patch status=%d body=%s", patchRes.StatusCode, patchBody)
	}
	revoked := readWSType(t, jiaConn, "session.revoked", 3*time.Second)
	if revoked["type"] != "session.revoked" {
		t.Fatalf("revoked = %#v", revoked)
	}

	// 乙断开最后一条连接 → 丙收到 presence online=false；last_seen_at 更新。
	beforeSeen := time.UnixMilli(0)
	var beforeMS int64
	_ = db.SQL().QueryRow(`SELECT last_seen_at FROM users WHERE id = ?`, yiID).Scan(&beforeMS)
	beforeSeen = time.UnixMilli(beforeMS)
	*clock = now.Add(time.Hour)

	_ = yiConn.Close(websocket.StatusNormalClosure, "")
	_ = readWSPresence(t, bingConn, yiID, false, 3*time.Second)
	var afterMS int64
	if err := db.SQL().QueryRow(`SELECT last_seen_at FROM users WHERE id = ?`, yiID).Scan(&afterMS); err != nil {
		t.Fatal(err)
	}
	afterSeen := time.UnixMilli(afterMS)
	if !afterSeen.After(beforeSeen) {
		t.Fatalf("last_seen_at not updated: before=%v after=%v", beforeSeen, afterSeen)
	}
}

func TestWSPing(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       hub.New(),
		DataDir:   t.TempDir(),
		MaxUpload: 1024 * 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "ws-ping-jia", "甲")
	conn := dialWS(t, srv.URL, jia.client)
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	payload, _ := json.Marshal(map[string]any{"type": "ping", "n": 42})
	writeWS(t, conn, payload)
	pong := readWSType(t, conn, "pong", 3*time.Second)
	if pong["n"] != float64(42) {
		t.Fatalf("pong n = %#v", pong["n"])
	}
}

func dialWS(t *testing.T, base string, c *http.Client) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/api/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: c})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	return conn
}

func writeWS(t *testing.T, c *websocket.Conn, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

func readWSType(t *testing.T, c *websocket.Conn, wantType string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		_, data, err := c.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("ws read waiting for %s: %v", wantType, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("ws json: %v data=%s", err, data)
		}
		if msg["type"] == wantType {
			return msg
		}
	}
	t.Fatalf("timeout waiting for type %s", wantType)
	return nil
}

func readWSPresence(t *testing.T, c *websocket.Conn, userID string, online bool, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		_, data, err := c.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("ws read waiting for presence %s online=%v: %v", userID, online, err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("ws json: %v data=%s", err, data)
		}
		if msg["type"] != "presence" {
			continue
		}
		if msg["userId"] == userID && msg["online"] == online {
			return msg
		}
	}
	t.Fatalf("timeout waiting for presence userId=%s online=%v", userID, online)
	return nil
}

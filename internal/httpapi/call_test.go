package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"superlocal/internal/auth"
	"superlocal/internal/call"
	"superlocal/internal/httpapi"
	"superlocal/internal/hub"
)

func TestCallSignaling(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	h := hub.New()
	calls := call.New(func() time.Time { return now }, func(time.Time) string { return "call-fixed" }, func(string) call.Turn {
		return call.Turn{URLs: []string{"stun:127.0.0.1:9"}, Username: "u", Credential: "p"}
	})
	srv := httptest.NewTLSServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       h,
		DataDir:   t.TempDir(),
		MaxUpload: 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
		Calls:     calls,
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "call-key-jia", "甲")
	yi := loginClient(t, srv.URL, "call-key-yi", "乙")
	bing := loginClient(t, srv.URL, "call-key-bing", "丙")
	yiID := getMe(t, yi.client, srv.URL)["id"].(string)

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
	var opened map[string]string
	if err := json.Unmarshal(openRaw, &opened); err != nil {
		t.Fatal(err)
	}
	convID := opened["id"]

	jiaConn := dialWS(t, srv.URL, jia.client)
	yiConn := dialWS(t, srv.URL, yi.client)
	bingConn := dialWS(t, srv.URL, bing.client)
	t.Cleanup(func() {
		_ = jiaConn.Close(websocket.StatusNormalClosure, "")
		_ = yiConn.Close(websocket.StatusNormalClosure, "")
		_ = bingConn.Close(websocket.StatusNormalClosure, "")
	})

	invite, _ := json.Marshal(map[string]string{"type": "call.invite", "conversationId": convID, "kind": "audio"})
	writeWS(t, jiaConn, invite)
	yiInvite := readWSType(t, yiConn, "call.invite", 3*time.Second)
	if yiInvite["callId"] == "" || yiInvite["fromNickname"] != "甲" {
		t.Fatalf("%#v", yiInvite)
	}
	_ = readWSType(t, jiaConn, "call.ringing", 3*time.Second)

	writeWS(t, bingConn, invite)
	denied := readWSType(t, bingConn, "call.reject", 3*time.Second)
	if denied["reason"] != "forbidden" {
		t.Fatalf("%#v", denied)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, data, err := yiConn.Read(waitCtx)
	if err == nil {
		var extra map[string]any
		_ = json.Unmarshal(data, &extra)
		if extra["type"] == "call.invite" {
			t.Fatalf("yi received a second invite: %s", data)
		}
	}

	_ = yiConn.Close(websocket.StatusNormalClosure, "")
	hung := readWSType(t, jiaConn, "call.hangup", 3*time.Second)
	if hung["callId"] == "" {
		t.Fatalf("%#v", hung)
	}
}

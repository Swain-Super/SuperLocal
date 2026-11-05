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
	"superlocal/internal/screen"
)

func TestScreenSignaling(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	db := openHTTPDB(t)
	h := hub.New()
	calls := call.New(func() time.Time { return now }, func(time.Time) string { return "call-fixed" }, func(string) call.Turn {
		return call.Turn{URLs: []string{"stun:127.0.0.1:9"}, Username: "u", Credential: "p"}
	})
	screens := screen.New(func() time.Time { return now }, func(time.Time) string { return "screen-fixed" }, func(string) screen.Turn {
		return screen.Turn{URLs: []string{"stun:127.0.0.1:9"}, Username: "u", Credential: "p"}
	})
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		Hub:       h,
		DataDir:   t.TempDir(),
		MaxUpload: 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return now },
		Calls:     calls,
		Screens:   screens,
	}))
	t.Cleanup(srv.Close)

	jia := loginClient(t, srv.URL, "screen-key-jia", "甲")
	yi := loginClient(t, srv.URL, "screen-key-yi", "乙")
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
	t.Cleanup(func() {
		_ = jiaConn.Close(websocket.StatusNormalClosure, "")
		_ = yiConn.Close(websocket.StatusNormalClosure, "")
	})

	if _, err := db.SQL().Exec(`UPDATE conversations SET type = 'group' WHERE id = ?`, convID); err != nil {
		t.Fatal(err)
	}
	groupInvite, _ := json.Marshal(map[string]string{"type": "screen.invite", "conversationId": convID})
	writeWS(t, jiaConn, groupInvite)
	denied := readWSType(t, jiaConn, "screen.reject", 3*time.Second)
	if denied["reason"] != "forbidden" {
		t.Fatalf("%#v", denied)
	}
	if _, err := db.SQL().Exec(`UPDATE conversations SET type = 'direct' WHERE id = ?`, convID); err != nil {
		t.Fatal(err)
	}

	callInvite, _ := json.Marshal(map[string]string{"type": "call.invite", "conversationId": convID, "kind": "audio"})
	writeWS(t, jiaConn, callInvite)
	if got := readWSType(t, yiConn, "call.invite", 3*time.Second); got["callId"] == "" {
		t.Fatalf("%#v", got)
	}
	_ = readWSType(t, jiaConn, "call.ringing", 3*time.Second)

	writeWS(t, jiaConn, groupInvite)
	yiInvite := readWSType(t, yiConn, "screen.invite", 3*time.Second)
	if yiInvite["screenId"] == "" || yiInvite["fromNickname"] != "甲" {
		t.Fatalf("%#v", yiInvite)
	}
	_ = readWSType(t, jiaConn, "screen.ringing", 3*time.Second)

	writeWS(t, jiaConn, groupInvite)
	busy := readWSType(t, jiaConn, "screen.reject", 3*time.Second)
	if busy["reason"] != "busy" {
		t.Fatalf("%#v", busy)
	}

	early, _ := json.Marshal(map[string]string{"type": "screen.offer", "screenId": "screen-fixed", "sdp": "early"})
	writeWS(t, jiaConn, early)
	waitCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, data, err := yiConn.Read(waitCtx)
	if err == nil {
		var extra map[string]any
		_ = json.Unmarshal(data, &extra)
		if extra["type"] == "screen.offer" {
			t.Fatalf("offer forwarded before accept: %s", data)
		}
	}

	_ = yiConn.Close(websocket.StatusNormalClosure, "")
	hung := readWSType(t, jiaConn, "screen.hangup", 3*time.Second)
	if hung["screenId"] == "" {
		t.Fatalf("%#v", hung)
	}
}

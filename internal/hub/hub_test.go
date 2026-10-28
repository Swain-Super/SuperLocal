package hub_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"superlocal/internal/hub"
)

func TestHubMultiConnAndKick(t *testing.T) {
	h := hub.New()
	if h == nil {
		t.Fatal("hub.New() returned nil")
	}

	c1srv, c1cli := pairWS(t)
	c2srv, c2cli := pairWS(t)
	t.Cleanup(func() {
		_ = c1cli.Close(websocket.StatusNormalClosure, "")
		_ = c2cli.Close(websocket.StatusNormalClosure, "")
		_ = c1srv.Close(websocket.StatusNormalClosure, "")
		_ = c2srv.Close(websocket.StatusNormalClosure, "")
	})

	h.Add("u1", c1srv)
	h.Add("u1", c2srv)

	online := h.OnlineIDs()
	if !online["u1"] {
		t.Fatalf("OnlineIDs after two Adds = %#v, want u1 online", online)
	}

	if offline := h.Remove("u1", c1srv); offline {
		t.Fatal("Remove first conn returned nowOffline=true, want false")
	}
	online = h.OnlineIDs()
	if !online["u1"] {
		t.Fatalf("still online after one Remove: %#v", online)
	}

	if offline := h.Remove("u1", c2srv); !offline {
		t.Fatal("Remove last conn returned nowOffline=false, want true")
	}
	online = h.OnlineIDs()
	if online["u1"] {
		t.Fatalf("offline after last Remove: %#v", online)
	}

	// Kick writes session.revoked then closes.
	c3srv, c3cli := pairWS(t)
	h.Add("u2", c3srv)
	h.Kick("u2")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := c3cli.Read(ctx)
	if err != nil {
		t.Fatalf("Kick read: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("Kick json: %v data=%s", err, data)
	}
	if msg["type"] != "session.revoked" {
		t.Fatalf("Kick type = %#v, want session.revoked", msg["type"])
	}
}

func pairWS(t *testing.T) (serverConn, clientConn *websocket.Conn) {
	t.Helper()
	ready := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("Accept: %v", err)
			return
		}
		ready <- c
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cli, _, err := websocket.Dial(ctx, "ws"+srv.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	select {
	case serverConn = <-ready:
	case <-ctx.Done():
		t.Fatal("timeout waiting for Accept")
	}
	return serverConn, cli
}

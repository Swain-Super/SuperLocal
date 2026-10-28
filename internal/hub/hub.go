package hub

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Hub 管理 WebSocket 连接与在线状态。
type Hub struct {
	mu    sync.Mutex
	conns map[string]map[*websocket.Conn]struct{}
}

// New 创建空 Hub。
func New() *Hub {
	return &Hub{conns: make(map[string]map[*websocket.Conn]struct{})}
}

// Add 登记用户的一条连接。
func (h *Hub) Add(userID string, c *websocket.Conn) {
	if h == nil || userID == "" || c == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.conns[userID]
	if set == nil {
		set = make(map[*websocket.Conn]struct{})
		h.conns[userID] = set
	}
	set[c] = struct{}{}
}

// Remove 移除一条连接。若该用户已无剩余连接则返回 nowOffline=true。
func (h *Hub) Remove(userID string, c *websocket.Conn) (nowOffline bool) {
	if h == nil || userID == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.conns[userID]
	if set == nil {
		return true
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.conns, userID)
		return true
	}
	return false
}

// OnlineIDs 返回当前在线用户。Hub 为 nil 时返回空 map。
func (h *Hub) OnlineIDs() map[string]bool {
	if h == nil {
		return map[string]bool{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]bool, len(h.conns))
	for id, set := range h.conns {
		if len(set) > 0 {
			out[id] = true
		}
	}
	return out
}

// Send 向指定用户的全部连接写入 payload。
func (h *Hub) Send(userIDs []string, payload []byte) {
	if h == nil || len(payload) == 0 {
		return
	}
	h.mu.Lock()
	targets := make([]*websocket.Conn, 0)
	for _, id := range userIDs {
		for c := range h.conns[id] {
			targets = append(targets, c)
		}
	}
	h.mu.Unlock()

	for _, c := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.Write(ctx, websocket.MessageText, payload)
		cancel()
	}
}

// Kick 向该用户全部连接写入 session.revoked 后关闭。Hub 为 nil 时为 no-op。
func (h *Hub) Kick(userID string) {
	if h == nil || userID == "" {
		return
	}
	payload := []byte(`{"type":"session.revoked"}`)
	h.mu.Lock()
	set := h.conns[userID]
	conns := make([]*websocket.Conn, 0, len(set))
	for c := range set {
		conns = append(conns, c)
	}
	delete(h.conns, userID)
	h.mu.Unlock()

	for _, c := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.Write(ctx, websocket.MessageText, payload)
		cancel()
		_ = c.Close(websocket.StatusPolicyViolation, "session revoked")
	}
}

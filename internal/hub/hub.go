package hub

// Hub 管理 WebSocket 连接。Task 12 仅提供 nil 安全的占位实现。
type Hub struct{}

// OnlineIDs 返回当前在线用户。Hub 为 nil 时返回空 map。
func (h *Hub) OnlineIDs() map[string]bool {
	if h == nil {
		return map[string]bool{}
	}
	return map[string]bool{}
}

// Kick 断开指定用户的全部连接。Hub 为 nil 时为 no-op。
func (h *Hub) Kick(userID string) {
	if h == nil {
		return
	}
}

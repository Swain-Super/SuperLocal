package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"superlocal/internal/store"
)

type wsIncoming struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	Kind           string `json:"kind"`
	Text           string `json:"text"`
	Body           string `json:"body"`
	FileID         string `json:"fileId"`
	ReplyToID      string `json:"replyToId"`
	MessageID      string `json:"messageId"`
	Emoji          string `json:"emoji"`
	N              int64  `json:"n"`
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if s.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "实时通道不可用")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	wasOffline := !s.hub.OnlineIDs()[user.ID]
	s.hub.Add(user.ID, conn)
	if wasOffline {
		s.broadcastPresence(user.ID, true)
	}

	defer func() {
		offline := s.hub.Remove(user.ID, conn)
		_ = conn.Close(websocket.StatusNormalClosure, "")
		if offline {
			_, _ = s.db.SQL().Exec(
				`UPDATE users SET last_seen_at = ? WHERE id = ?`,
				s.now().UnixMilli(), user.ID,
			)
			s.broadcastPresence(user.ID, false)
		}
	}()

	ctx := r.Context()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var in wsIncoming
		if err := json.Unmarshal(data, &in); err != nil {
			s.writeWSError(conn, "请求无效")
			continue
		}
		switch in.Type {
		case "message.send":
			s.handleWSMessageSend(conn, user, in)
		case "message.recall":
			s.handleWSRecall(conn, user, in)
		case "message.react":
			s.handleWSReact(conn, user, in)
		case "ping":
			s.writeWSPong(conn, in.N)
		default:
			s.writeWSError(conn, "未知事件")
		}
	}
}

func (s *server) handleWSMessageSend(conn *websocket.Conn, user store.User, in wsIncoming) {
	kind := in.Kind
	if kind == "" {
		kind = "text"
	}
	body := in.Text
	if body == "" {
		body = in.Body
	}
	msg, err := s.db.PostReply(user.ID, in.ConversationID, kind, body, in.FileID, in.ReplyToID, s.now())
	if err != nil {
		s.writeWSError(conn, err.Error())
		return
	}
	s.pushMessage(conn, in.ConversationID, "message.new", msg)
}

func (s *server) handleWSRecall(conn *websocket.Conn, user store.User, in wsIncoming) {
	msg, err := s.db.RecallMessage(user.ID, in.MessageID, s.now())
	if err != nil {
		s.writeWSError(conn, err.Error())
		return
	}
	s.pushMessage(conn, msg.ConversationID, "message.updated", msg)
}

func (s *server) handleWSReact(conn *websocket.Conn, user store.User, in wsIncoming) {
	msg, err := s.db.ToggleReaction(user.ID, in.MessageID, in.Emoji, s.now())
	if err != nil {
		s.writeWSError(conn, err.Error())
		return
	}
	s.pushMessage(conn, msg.ConversationID, "message.updated", msg)
}

func (s *server) pushMessage(conn *websocket.Conn, convID, eventType string, msg store.Message) {
	memberIDs, err := s.conversationMemberIDs(convID)
	if err != nil {
		s.writeWSError(conn, "推送失败")
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":    eventType,
		"message": msg,
	})
	if err != nil {
		s.writeWSError(conn, "推送失败")
		return
	}
	s.hub.Send(memberIDs, payload)
}

func (s *server) conversationMemberIDs(convID string) ([]string, error) {
	rows, err := s.db.SQL().Query(
		`SELECT user_id FROM members WHERE conversation_id = ?`, convID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *server) broadcastDnd(userID string, quiet bool) {
	if s.hub == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":   "dnd",
		"userId": userID,
		"dnd":    quiet,
	})
	if err != nil {
		return
	}
	ids := make([]string, 0)
	for id := range s.hub.OnlineIDs() {
		ids = append(ids, id)
	}
	s.hub.Send(ids, payload)
}

func (s *server) broadcastPresence(userID string, online bool) {
	if s.hub == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"type":   "presence",
		"userId": userID,
		"online": online,
	})
	if err != nil {
		return
	}
	ids := make([]string, 0)
	for id := range s.hub.OnlineIDs() {
		ids = append(ids, id)
	}
	// 下线时本人已不在 OnlineIDs；仍把 payload 发给当前在线用户即可。
	s.hub.Send(ids, payload)
}

func (s *server) writeWSPong(conn *websocket.Conn, n int64) {
	payload, err := json.Marshal(map[string]any{
		"type": "pong",
		"n":    n,
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Write(ctx, websocket.MessageText, payload)
}

func (s *server) writeWSError(conn *websocket.Conn, msg string) {
	payload, err := json.Marshal(map[string]string{
		"type":  "error",
		"error": msg,
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = conn.Write(ctx, websocket.MessageText, payload)
}

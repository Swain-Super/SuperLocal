package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	SenderID       string    `json:"senderId"`
	SenderNickname string    `json:"senderNickname"`
	Kind           string    `json:"kind"`
	Body           string    `json:"body"`
	FileID         string    `json:"fileId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type ConversationView struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Title       string      `json:"title"`
	Peer        *PublicUser `json:"peer,omitempty"`
	MemberCount int         `json:"memberCount"`
	Unread      int         `json:"unread"`
	LastBody    string      `json:"lastBody"`
	LastAt      time.Time   `json:"lastAt"`
}

// directKey 把两个用户 id 按字符串排序后用 | 连接，保证同一对用户只有一个键。
func directKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "|" + b
}

func (db *DB) OpenDirect(selfID, peerID string, now time.Time) (string, error) {
	if selfID == "" || peerID == "" || selfID == peerID {
		return "", fmt.Errorf("一对一需要两个不同的用户")
	}
	key := directKey(selfID, peerID)
	existing, err := db.conversationIDByDirectKey(key)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	id := NewID(now)
	err = db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		if _, txErr = tx.Exec(
			`INSERT INTO conversations (id, type, direct_key, created_at) VALUES (?, 'direct', ?, ?)`,
			id, key, now.UnixMilli(),
		); txErr != nil {
			return txErr
		}
		for _, userID := range []string{selfID, peerID} {
			if _, txErr = tx.Exec(
				`INSERT INTO members (conversation_id, user_id, joined_at) VALUES (?, ?, ?)`,
				id, userID, now.UnixMilli(),
			); txErr != nil {
				return txErr
			}
		}
		return tx.Commit()
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return db.conversationIDByDirectKey(key)
		}
		return "", err
	}
	return id, nil
}

func (db *DB) conversationIDByDirectKey(key string) (string, error) {
	var id string
	err := db.sql.QueryRow(`SELECT id FROM conversations WHERE direct_key = ?`, key).Scan(&id)
	return id, err
}

func (db *DB) PostMessage(senderID, convID, kind, body, fileID string, now time.Time) (Message, error) {
	sender, err := db.UserByID(senderID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, fmt.Errorf("不是会话成员")
		}
		return Message{}, err
	}
	if sender.Banned {
		return Message{}, fmt.Errorf("账号已被封禁")
	}
	if !sender.CanSendMessage {
		return Message{}, fmt.Errorf("没有发言权限")
	}

	var convType string
	err = db.sql.QueryRow(
		`SELECT type FROM conversations WHERE id = ? AND dissolved = 0`, convID,
	).Scan(&convType)
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, fmt.Errorf("会话不存在")
		}
		return Message{}, err
	}
	var member int
	if err = db.sql.QueryRow(
		`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`,
		convID, senderID,
	).Scan(&member); err != nil {
		return Message{}, err
	}
	if member == 0 {
		return Message{}, fmt.Errorf("不是会话成员")
	}
	if kind == "text" {
		if strings.TrimSpace(body) == "" {
			return Message{}, fmt.Errorf("文字不能为空")
		}
		if utf8.RuneCountInString(body) > 4000 {
			return Message{}, fmt.Errorf("文字最长 4000 字")
		}
	}
	if convType == "direct" {
		var banned int
		err = db.sql.QueryRow(
			`SELECT u.banned FROM members m
			 JOIN users u ON u.id = m.user_id
			 WHERE m.conversation_id = ? AND m.user_id != ?`,
			convID, senderID,
		).Scan(&banned)
		if err != nil {
			return Message{}, err
		}
		if banned != 0 {
			return Message{}, fmt.Errorf("对方已被封禁")
		}
	}

	id := NewID(now)
	err = db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`INSERT INTO messages (id, conversation_id, sender_id, kind, body, file_id, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, convID, senderID, kind, body, fileID, now.UnixMilli(),
		)
		return execErr
	})
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:             id,
		ConversationID: convID,
		SenderID:       senderID,
		SenderNickname: sender.Nickname,
		Kind:           kind,
		Body:           body,
		FileID:         fileID,
		CreatedAt:      unixMilliTime(now.UnixMilli()),
	}, nil
}

func (db *DB) ListConversations(userID string, online map[string]bool) ([]ConversationView, error) {
	rows, err := db.sql.Query(`
		SELECT c.id, c.type, c.name,
			(SELECT COUNT(*) FROM members m WHERE m.conversation_id = c.id),
			COALESCE((SELECT body FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1), ''),
			COALESCE((SELECT created_at FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1), 0),
			(SELECT COUNT(*) FROM messages msg
				WHERE msg.conversation_id = c.id
					AND msg.id > self.last_read_id
					AND msg.sender_id != self.user_id)
		FROM conversations c
		JOIN members self ON self.conversation_id = c.id AND self.user_id = ?
		WHERE c.dissolved = 0
			AND (c.type != 'direct' OR EXISTS (
				SELECT 1 FROM messages m WHERE m.conversation_id = c.id
			))
		ORDER BY COALESCE((SELECT created_at FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1), 0) DESC, c.id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type pendingView struct {
		view ConversationView
		name string
	}
	pending := []pendingView{}
	for rows.Next() {
		var item pendingView
		var lastAt int64
		if err := rows.Scan(
			&item.view.ID, &item.view.Type, &item.name,
			&item.view.MemberCount, &item.view.LastBody, &lastAt, &item.view.Unread,
		); err != nil {
			return nil, err
		}
		item.view.LastAt = unixMilliTime(lastAt)
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	out := make([]ConversationView, 0, len(pending))
	for _, item := range pending {
		view := item.view
		if view.Type == "direct" {
			peer, err := db.peerUser(view.ID, userID)
			if err != nil {
				return nil, err
			}
			pub := Public(peer)
			if online[peer.ID] {
				pub.Online = true
			}
			view.Peer = &pub
			view.Title = peer.Nickname
		} else {
			view.Title = item.name
		}
		out = append(out, view)
	}
	return out, nil
}

func (db *DB) peerUser(convID, selfID string) (User, error) {
	var peerID string
	err := db.sql.QueryRow(
		`SELECT user_id FROM members WHERE conversation_id = ? AND user_id != ? LIMIT 1`,
		convID, selfID,
	).Scan(&peerID)
	if err != nil {
		return User{}, err
	}
	return db.UserByID(peerID)
}

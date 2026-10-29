package store

import (
	"database/sql"
	"fmt"
	"time"
)

const recallWindow = 24 * time.Hour

var reactionEmoji = map[string]struct{}{
	"thumb": {},
	"ok":    {},
	"plus":  {},
	"done":  {},
	"get":   {},
	"smile": {},
}

func hideRecalled(msg Message) Message {
	if !msg.Recalled {
		return msg
	}
	msg.Body = ""
	msg.FileID = ""
	msg.FileName = ""
	msg.FileSize = 0
	msg.Mime = ""
	return msg
}

func (db *DB) replyView(messageID string) (*ReplyView, error) {
	var view ReplyView
	var recalled int
	var fileName string
	err := db.sql.QueryRow(`
		SELECT m.id, COALESCE(u.nickname, ''), m.kind, m.body, m.recalled, COALESCE(f.name, '')
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		LEFT JOIN files f ON f.id = m.file_id AND m.file_id != ''
		WHERE m.id = ?`, messageID).Scan(
		&view.ID, &view.SenderNickname, &view.Kind, &view.Body, &recalled, &fileName,
	)
	if err != nil {
		return nil, err
	}
	view.Recalled = recalled != 0
	view.FileName = fileName
	if view.Recalled {
		view.Body = ""
		view.FileName = ""
	}
	return &view, nil
}

func (db *DB) attachMessageExtras(userID string, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(msgs))
	index := map[string]int{}
	replyIDs := map[string]struct{}{}
	for i, msg := range msgs {
		ids = append(ids, msg.ID)
		index[msg.ID] = i
		if msg.ReplyToID != "" {
			replyIDs[msg.ReplyToID] = struct{}{}
		}
	}
	for id := range replyIDs {
		view, err := db.replyView(id)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return err
		}
		for i := range msgs {
			if msgs[i].ReplyToID == id {
				msgs[i].Reply = view
			}
		}
	}
	placeholders := ""
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}
	rows, err := db.sql.Query(
		`SELECT message_id, emoji, user_id FROM reactions WHERE message_id IN (`+placeholders+`) ORDER BY emoji, user_id`,
		args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	type bucket struct {
		count int
		self  bool
	}
	grouped := map[string]map[string]*bucket{}
	for rows.Next() {
		var messageID, emoji, reactor string
		if err := rows.Scan(&messageID, &emoji, &reactor); err != nil {
			return err
		}
		byEmoji := grouped[messageID]
		if byEmoji == nil {
			byEmoji = map[string]*bucket{}
			grouped[messageID] = byEmoji
		}
		item := byEmoji[emoji]
		if item == nil {
			item = &bucket{}
			byEmoji[emoji] = item
		}
		item.count++
		if reactor == userID {
			item.self = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	order := []string{"thumb", "ok", "plus", "done", "get", "smile"}
	for messageID, byEmoji := range grouped {
		i, ok := index[messageID]
		if !ok {
			continue
		}
		for _, emoji := range order {
			item := byEmoji[emoji]
			if item == nil {
				continue
			}
			msgs[i].Reactions = append(msgs[i].Reactions, ReactionView{
				Emoji: emoji,
				Count: item.count,
				Self:  item.self,
			})
		}
	}
	return nil
}

func (db *DB) messageForUser(userID, messageID string) (Message, error) {
	var msg Message
	var created int64
	var recalled int
	err := db.sql.QueryRow(`
		SELECT m.id, m.conversation_id, m.sender_id, COALESCE(u.nickname, ''), m.kind, m.body, m.file_id, m.created_at,
			COALESCE(m.reply_to_id, ''), m.recalled,
			COALESCE(f.name, ''), COALESCE(f.mime, ''), COALESCE(f.size, 0)
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		LEFT JOIN files f ON f.id = m.file_id AND m.file_id != ''
		WHERE m.id = ?`, messageID).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.SenderNickname,
		&msg.Kind, &msg.Body, &msg.FileID, &created,
		&msg.ReplyToID, &recalled,
		&msg.FileName, &msg.Mime, &msg.FileSize,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Message{}, fmt.Errorf("消息不存在")
		}
		return Message{}, err
	}
	member, err := db.isMember(msg.ConversationID, userID)
	if err != nil {
		return Message{}, err
	}
	if !member {
		return Message{}, fmt.Errorf("不是会话成员")
	}
	msg.CreatedAt = unixMilliTime(created)
	msg.Recalled = recalled != 0
	list := []Message{hideRecalled(msg)}
	if err := db.attachMessageExtras(userID, list); err != nil {
		return Message{}, err
	}
	return list[0], nil
}

func (db *DB) RecallMessage(userID, messageID string, now time.Time) (Message, error) {
	msg, err := db.messageForUser(userID, messageID)
	if err != nil {
		return Message{}, err
	}
	if msg.SenderID != userID {
		return Message{}, fmt.Errorf("只能撤回自己的消息")
	}
	if msg.Recalled {
		return msg, nil
	}
	if now.Sub(msg.CreatedAt) > recallWindow {
		return Message{}, fmt.Errorf("已超过撤回时间")
	}
	err = db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(`UPDATE messages SET recalled = 1 WHERE id = ? AND sender_id = ?`, messageID, userID)
		return execErr
	})
	if err != nil {
		return Message{}, err
	}
	return db.messageForUser(userID, messageID)
}

func (db *DB) ToggleReaction(userID, messageID, emoji string, now time.Time) (Message, error) {
	if _, ok := reactionEmoji[emoji]; !ok {
		return Message{}, fmt.Errorf("不支持这个表情")
	}
	if _, err := db.messageForUser(userID, messageID); err != nil {
		return Message{}, err
	}
	err := db.withLockRetry(func() error {
		res, execErr := db.sql.Exec(
			`DELETE FROM reactions WHERE message_id = ? AND user_id = ? AND emoji = ?`,
			messageID, userID, emoji,
		)
		if execErr != nil {
			return execErr
		}
		n, execErr := res.RowsAffected()
		if execErr != nil {
			return execErr
		}
		if n > 0 {
			return nil
		}
		_, execErr = db.sql.Exec(
			`INSERT INTO reactions (message_id, user_id, emoji, created_at) VALUES (?, ?, ?, ?)`,
			messageID, userID, emoji, now.UnixMilli(),
		)
		return execErr
	})
	if err != nil {
		return Message{}, err
	}
	return db.messageForUser(userID, messageID)
}

func (db *DB) SetConversationFlags(userID, convID string, pinned, muted *bool) error {
	member, err := db.isMember(convID, userID)
	if err != nil {
		return err
	}
	if !member {
		return fmt.Errorf("不是会话成员")
	}
	return db.withLockRetry(func() error {
		if pinned != nil {
			value := 0
			if *pinned {
				value = 1
			}
			if _, err := db.sql.Exec(
				`UPDATE members SET pinned = ? WHERE conversation_id = ? AND user_id = ?`,
				value, convID, userID,
			); err != nil {
				return err
			}
		}
		if muted != nil {
			value := 0
			if *muted {
				value = 1
			}
			if _, err := db.sql.Exec(
				`UPDATE members SET muted = ? WHERE conversation_id = ? AND user_id = ?`,
				value, convID, userID,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (db *DB) Members(userID, convID string, online map[string]bool) ([]PublicUser, error) {
	member, err := db.isMember(convID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, fmt.Errorf("不是会话成员")
	}
	rows, err := db.sql.Query(`
		SELECT u.id, u.nickname, u.avatar_path, u.signature, u.pendant, u.dnd
		FROM members m
		JOIN users u ON u.id = m.user_id
		WHERE m.conversation_id = ?
		ORDER BY u.nickname, u.id`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PublicUser{}
	for rows.Next() {
		var id, nickname, avatar, signature, pendant string
		var quiet int
		if err := rows.Scan(&id, &nickname, &avatar, &signature, &pendant, &quiet); err != nil {
			return nil, err
		}
		pub := Public(User{
			ID: id, Nickname: nickname, AvatarPath: avatar,
			Signature: signature, Pendant: pendant, Dnd: quiet != 0,
		})
		pub.Online = online[id]
		out = append(out, pub)
	}
	return out, rows.Err()
}

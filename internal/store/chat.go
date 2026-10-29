package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Message struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversationId"`
	SenderID       string         `json:"senderId"`
	SenderNickname string         `json:"senderNickname"`
	Kind           string         `json:"kind"`
	Body           string         `json:"body"`
	FileID         string         `json:"fileId"`
	FileName       string         `json:"fileName"`
	FileSize       int64          `json:"fileSize"`
	Mime           string         `json:"mime"`
	CreatedAt      time.Time      `json:"createdAt"`
	ReplyToID      string         `json:"replyToId,omitempty"`
	Reply          *ReplyView     `json:"reply,omitempty"`
	Recalled       bool           `json:"recalled"`
	Reactions      []ReactionView `json:"reactions,omitempty"`
}

type ReplyView struct {
	ID             string `json:"id"`
	SenderNickname string `json:"senderNickname"`
	Kind           string `json:"kind"`
	Body           string `json:"body"`
	FileName       string `json:"fileName,omitempty"`
	Recalled       bool   `json:"recalled"`
}

type ReactionView struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
	Self  bool   `json:"self"`
}

type ConversationView struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	OwnerID     string      `json:"ownerId"`
	Title       string      `json:"title"`
	Peer        *PublicUser `json:"peer,omitempty"`
	MemberCount int         `json:"memberCount"`
	Unread      int         `json:"unread"`
	LastBody    string      `json:"lastBody"`
	LastAt      time.Time   `json:"lastAt"`
	Pinned      bool        `json:"pinned"`
	Muted       bool        `json:"muted"`
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
	peer, err := db.UserByID(peerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("用户不存在")
		}
		return "", err
	}
	if peer.Banned {
		return "", fmt.Errorf("用户已被封禁")
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
	return db.postMessage(senderID, convID, kind, body, fileID, "", now)
}

func (db *DB) PostReply(senderID, convID, kind, body, fileID, replyToID string, now time.Time) (Message, error) {
	return db.postMessage(senderID, convID, kind, body, fileID, replyToID, now)
}

func (db *DB) postMessage(senderID, convID, kind, body, fileID, replyToID string, now time.Time) (Message, error) {
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
	if (kind == "image" || kind == "file") && !sender.CanSendFile {
		return Message{}, fmt.Errorf("没有发文件权限")
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
	if replyToID != "" {
		var replyConv string
		err = db.sql.QueryRow(`SELECT conversation_id FROM messages WHERE id = ?`, replyToID).Scan(&replyConv)
		if err != nil || replyConv != convID {
			return Message{}, fmt.Errorf("引用的消息不存在")
		}
	}
	if kind == "text" {
		if strings.TrimSpace(body) == "" {
			return Message{}, fmt.Errorf("文字不能为空")
		}
		if utf8.RuneCountInString(body) > 4000 {
			return Message{}, fmt.Errorf("文字最长 4000 字")
		}
		// 文字消息不得挂文件，避免露出他人文件名或误绑。
		fileID = ""
	} else if kind == "image" || kind == "file" {
		if fileID == "" {
			return Message{}, fmt.Errorf("缺少文件")
		}
	}
	if convType == "direct" {
		var banned, quiet int
		err = db.sql.QueryRow(
			`SELECT u.banned, u.dnd FROM members m
			 JOIN users u ON u.id = m.user_id
			 WHERE m.conversation_id = ? AND m.user_id != ?`,
			convID, senderID,
		).Scan(&banned, &quiet)
		if err != nil {
			return Message{}, err
		}
		if banned != 0 {
			return Message{}, fmt.Errorf("对方已被封禁")
		}
		if quiet != 0 {
			return Message{}, fmt.Errorf("对方已开启免打扰")
		}
	}

	id := NewID(now)
	var fileName, mime string
	var fileSize int64
	err = db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		if _, txErr = tx.Exec(
			`INSERT INTO messages (id, conversation_id, sender_id, kind, body, file_id, created_at, reply_to_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, convID, senderID, kind, body, fileID, now.UnixMilli(), replyToID,
		); txErr != nil {
			return txErr
		}
		// 图片和文件与 BindFile 使用同一条更新：必须属于发送者，且 message_id 仍为空。
		if kind == "image" || kind == "file" {
			if txErr = bindOutgoing(tx, senderID, kind, fileID, id); txErr != nil {
				return txErr
			}
			txErr = tx.QueryRow(
				`SELECT name, mime, size FROM files WHERE id = ?`, fileID,
			).Scan(&fileName, &mime, &fileSize)
			if txErr != nil {
				return txErr
			}
		}
		return tx.Commit()
	})
	if err != nil {
		return Message{}, err
	}
	msg := Message{
		ID:             id,
		ConversationID: convID,
		SenderID:       senderID,
		SenderNickname: sender.Nickname,
		Kind:           kind,
		Body:           body,
		FileID:         fileID,
		FileName:       fileName,
		FileSize:       fileSize,
		Mime:           mime,
		CreatedAt:      unixMilliTime(now.UnixMilli()),
		ReplyToID:      replyToID,
	}
	if replyToID != "" {
		if reply, err := db.replyView(replyToID); err == nil {
			msg.Reply = reply
		}
	}
	return msg, nil
}

func (db *DB) ListConversations(userID string, online map[string]bool) ([]ConversationView, error) {
	rows, err := db.sql.Query(`
		SELECT c.id, c.type, c.owner_id, c.name,
			(SELECT COUNT(*) FROM members m WHERE m.conversation_id = c.id),
			COALESCE((
				SELECT CASE WHEN messages.recalled = 1 THEN '撤回了一条消息'
					ELSE COALESCE(
						(SELECT name FROM files WHERE id = messages.file_id AND messages.file_id != ''),
						messages.body
					)
				END
				FROM messages
				WHERE conversation_id = c.id
				ORDER BY id DESC
				LIMIT 1
			), ''),
			COALESCE((SELECT created_at FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1), 0),
			(SELECT COUNT(*) FROM messages msg
				WHERE msg.conversation_id = c.id
					AND msg.id > self.last_read_id
					AND msg.sender_id != self.user_id),
			self.pinned, self.muted
		FROM conversations c
		JOIN members self ON self.conversation_id = c.id AND self.user_id = ?
		WHERE c.dissolved = 0
			AND (c.type != 'direct' OR EXISTS (
				SELECT 1 FROM messages m WHERE m.conversation_id = c.id
			))
		ORDER BY self.pinned DESC, COALESCE((SELECT created_at FROM messages WHERE conversation_id = c.id ORDER BY id DESC LIMIT 1), 0) DESC, c.id DESC
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
		var pinned, muted int
		if err := rows.Scan(
			&item.view.ID, &item.view.Type, &item.view.OwnerID, &item.name,
			&item.view.MemberCount, &item.view.LastBody, &lastAt, &item.view.Unread,
			&pinned, &muted,
		); err != nil {
			return nil, err
		}
		item.view.LastAt = unixMilliTime(lastAt)
		item.view.Pinned = pinned != 0
		item.view.Muted = muted != 0
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
				if err == sql.ErrNoRows {
					continue
				}
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

func normalizeGroupName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("群名不能为空")
	}
	if utf8.RuneCountInString(name) > 40 {
		return "", fmt.Errorf("群名最长 40 字")
	}
	return name, nil
}

// groupMemberIDs 把创建者放在首位，并去掉空值和重复成员。
func groupMemberIDs(ownerID string, memberIDs []string) ([]string, error) {
	seen := map[string]bool{ownerID: true}
	out := []string{ownerID}
	for _, id := range memberIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("至少再选择一名成员")
	}
	return out, nil
}

func (db *DB) CreateGroup(ownerID, name string, memberIDs []string, now time.Time) (string, error) {
	name, err := normalizeGroupName(name)
	if err != nil {
		return "", err
	}
	owner, err := db.UserByID(ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("没有建群权限")
		}
		return "", err
	}
	if !owner.CanCreateGroup {
		return "", fmt.Errorf("没有建群权限")
	}
	members, err := groupMemberIDs(ownerID, memberIDs)
	if err != nil {
		return "", err
	}
	for _, userID := range members[1:] {
		u, userErr := db.UserByID(userID)
		if userErr != nil {
			if userErr == sql.ErrNoRows {
				return "", fmt.Errorf("成员不存在")
			}
			return "", userErr
		}
		if u.Banned {
			return "", fmt.Errorf("成员已被封禁")
		}
	}

	id := NewID(now)
	err = db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		// 群的 direct_key 使用自己的 id，避免默认空字符串违反 UNIQUE。
		if _, txErr = tx.Exec(
			`INSERT INTO conversations (id, type, name, owner_id, direct_key, created_at) VALUES (?, 'group', ?, ?, ?, ?)`,
			id, name, ownerID, id, now.UnixMilli(),
		); txErr != nil {
			return txErr
		}
		for _, userID := range members {
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
		return "", err
	}
	return id, nil
}

func (db *DB) AddMembers(actorID, convID string, memberIDs []string, now time.Time) error {
	var convType, ownerID string
	var dissolved int
	err := db.sql.QueryRow(
		`SELECT type, owner_id, dissolved FROM conversations WHERE id = ?`, convID,
	).Scan(&convType, &ownerID, &dissolved)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("会话不存在")
		}
		return err
	}
	if dissolved != 0 || convType != "group" {
		return fmt.Errorf("会话不存在")
	}
	if actorID != ownerID {
		return fmt.Errorf("只有创建者可以添加成员")
	}
	member, err := db.isMember(convID, actorID)
	if err != nil {
		return err
	}
	if !member {
		return fmt.Errorf("只有创建者可以添加成员")
	}

	seen := map[string]bool{}
	fresh := []string{}
	for _, userID := range memberIDs {
		if userID == "" || userID == actorID || seen[userID] {
			continue
		}
		seen[userID] = true
		already, memberErr := db.isMember(convID, userID)
		if memberErr != nil {
			return memberErr
		}
		if already {
			continue
		}
		u, userErr := db.UserByID(userID)
		if userErr != nil {
			if userErr == sql.ErrNoRows {
				return fmt.Errorf("成员不存在")
			}
			return userErr
		}
		if u.Banned {
			return fmt.Errorf("成员已被封禁")
		}
		fresh = append(fresh, userID)
	}
	if len(fresh) == 0 {
		return nil
	}
	return db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		// 提交前再确认群状态与创建者仍在成员表，避免 Leave/Dissolve 与加人交错。
		var txType, txOwner string
		var txDissolved int
		txErr = tx.QueryRow(
			`SELECT type, owner_id, dissolved FROM conversations WHERE id = ?`, convID,
		).Scan(&txType, &txOwner, &txDissolved)
		if txErr != nil {
			if txErr == sql.ErrNoRows {
				return fmt.Errorf("会话不存在")
			}
			return txErr
		}
		if txDissolved != 0 || txType != "group" {
			return fmt.Errorf("会话不存在")
		}
		if actorID != txOwner {
			return fmt.Errorf("只有创建者可以添加成员")
		}
		var actorMember int
		txErr = tx.QueryRow(
			`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`,
			convID, actorID,
		).Scan(&actorMember)
		if txErr != nil {
			return txErr
		}
		if actorMember == 0 {
			return fmt.Errorf("只有创建者可以添加成员")
		}
		for _, userID := range fresh {
			if _, txErr = tx.Exec(
				`INSERT INTO members (conversation_id, user_id, joined_at) VALUES (?, ?, ?)`,
				convID, userID, now.UnixMilli(),
			); txErr != nil {
				return txErr
			}
		}
		return tx.Commit()
	})
}

func (db *DB) RemoveMember(actorID, convID, targetID string) error {
	if actorID == "" || targetID == "" || actorID == targetID {
		return fmt.Errorf("不能移出自己")
	}
	var convType, ownerID string
	var dissolved int
	err := db.sql.QueryRow(
		`SELECT type, owner_id, dissolved FROM conversations WHERE id = ?`, convID,
	).Scan(&convType, &ownerID, &dissolved)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("会话不存在")
		}
		return err
	}
	if dissolved != 0 || convType != "group" {
		return fmt.Errorf("会话不存在")
	}
	if actorID != ownerID {
		return fmt.Errorf("只有创建者可以移出成员")
	}
	member, err := db.isMember(convID, targetID)
	if err != nil {
		return err
	}
	if !member {
		return fmt.Errorf("不是会话成员")
	}
	return db.withLockRetry(func() error {
		res, execErr := db.sql.Exec(
			`DELETE FROM members WHERE conversation_id = ? AND user_id = ?`,
			convID, targetID,
		)
		if execErr != nil {
			return execErr
		}
		n, execErr := res.RowsAffected()
		if execErr != nil {
			return execErr
		}
		if n == 0 {
			return fmt.Errorf("不是会话成员")
		}
		return nil
	})
}

func (db *DB) Leave(userID, convID string) error {
	return db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		var group int
		txErr = tx.QueryRow(
			`SELECT 1 FROM conversations WHERE id = ? AND type = 'group' AND dissolved = 0`,
			convID,
		).Scan(&group)
		if txErr != nil {
			if txErr == sql.ErrNoRows {
				return fmt.Errorf("会话不存在")
			}
			return txErr
		}
		res, txErr := tx.Exec(
			`DELETE FROM members WHERE conversation_id = ? AND user_id = ?`,
			convID, userID,
		)
		if txErr != nil {
			return txErr
		}
		n, txErr := res.RowsAffected()
		if txErr != nil {
			return txErr
		}
		if n == 0 {
			return fmt.Errorf("不是会话成员")
		}
		return tx.Commit()
	})
}

func (db *DB) Dissolve(convID string) error {
	return db.withLockRetry(func() error {
		tx, txErr := db.sql.Begin()
		if txErr != nil {
			return txErr
		}
		defer tx.Rollback()
		res, txErr := tx.Exec(
			`UPDATE conversations SET dissolved = 1 WHERE id = ? AND type = 'group' AND dissolved = 0`,
			convID,
		)
		if txErr != nil {
			return txErr
		}
		n, txErr := res.RowsAffected()
		if txErr != nil {
			return txErr
		}
		if n == 0 {
			return fmt.Errorf("会话不存在")
		}
		if _, txErr = tx.Exec(`DELETE FROM members WHERE conversation_id = ?`, convID); txErr != nil {
			return txErr
		}
		return tx.Commit()
	})
}

func (db *DB) Messages(userID, convID, afterID string) ([]Message, error) {
	member, err := db.isMember(convID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, fmt.Errorf("不是会话成员")
	}
	rows, err := db.sql.Query(`
		SELECT m.id, m.conversation_id, m.sender_id, COALESCE(u.nickname, ''), m.kind, m.body, m.file_id, m.created_at,
			COALESCE(m.reply_to_id, ''), m.recalled,
			COALESCE(f.name, ''), COALESCE(f.mime, ''), COALESCE(f.size, 0)
		FROM messages m
		LEFT JOIN users u ON u.id = m.sender_id
		LEFT JOIN files f ON f.id = m.file_id AND m.file_id != ''
		WHERE m.conversation_id = ? AND m.id > ?
		ORDER BY m.id ASC`, convID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var msg Message
		var created int64
		var recalled int
		if err := rows.Scan(
			&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.SenderNickname,
			&msg.Kind, &msg.Body, &msg.FileID, &created,
			&msg.ReplyToID, &recalled,
			&msg.FileName, &msg.Mime, &msg.FileSize,
		); err != nil {
			return nil, err
		}
		msg.CreatedAt = unixMilliTime(created)
		msg.Recalled = recalled != 0
		out = append(out, hideRecalled(msg))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := db.attachMessageExtras(userID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *DB) MarkRead(userID, convID, messageID string) error {
	member, err := db.isMember(convID, userID)
	if err != nil {
		return err
	}
	if !member {
		return fmt.Errorf("不是会话成员")
	}
	var msgConv string
	err = db.sql.QueryRow(`SELECT conversation_id FROM messages WHERE id = ?`, messageID).Scan(&msgConv)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("消息不属于该会话")
		}
		return err
	}
	if msgConv != convID {
		return fmt.Errorf("消息不属于该会话")
	}
	return db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`UPDATE members SET last_read_id = ? WHERE conversation_id = ? AND user_id = ?`,
			messageID, convID, userID,
		)
		return execErr
	})
}

func (db *DB) SearchUsers(query string) ([]PublicUser, error) {
	rows, err := db.sql.Query(`
		SELECT id, nickname, avatar_path, signature, pendant, dnd FROM users
		WHERE banned = 0 AND nickname LIKE '%' || ? || '%'
		ORDER BY nickname, id
		LIMIT 50`, query)
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
		out = append(out, Public(User{ID: id, Nickname: nickname, AvatarPath: avatar, Signature: signature, Pendant: pendant, Dnd: quiet != 0}))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *DB) isMember(convID, userID string) (bool, error) {
	var n int
	err := db.sql.QueryRow(
		`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`,
		convID, userID,
	).Scan(&n)
	return n != 0, err
}

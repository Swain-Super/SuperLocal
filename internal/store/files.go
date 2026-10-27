package store

import (
	"database/sql"
	"fmt"
	"time"
)

type File struct {
	ID, OwnerID, Name, Mime, Path, MessageID string
	Size                                     int64
	CreatedAt                                time.Time
}

func (db *DB) InsertFile(ownerID, name, mime, path string, size int64, now time.Time) (string, error) {
	id := NewID(now)
	err := db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`INSERT INTO files (id, owner_id, name, mime, size, path, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, ownerID, name, mime, size, path, now.UnixMilli(),
		)
		return execErr
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// BindFile 把文件绑到消息。文件必须属于 ownerID，且 message_id 仍为空。
func (db *DB) BindFile(fileID, messageID, ownerID string) error {
	return db.withLockRetry(func() error {
		return bindFile(db.sql, fileID, messageID, ownerID)
	})
}

type sqlExec interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func bindFile(ex sqlExec, fileID, messageID, ownerID string) error {
	res, err := ex.Exec(
		`UPDATE files SET message_id = ? WHERE id = ? AND owner_id = ? AND message_id = ''`,
		messageID, fileID, ownerID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("文件不可用")
	}
	return nil
}

// bindOutgoing 在发送图片或文件时调用与 BindFile 相同的更新，并检查图片 MIME。
func bindOutgoing(tx *sql.Tx, senderID, kind, fileID, messageID string) error {
	var owner, mime, bound string
	err := tx.QueryRow(
		`SELECT owner_id, mime, message_id FROM files WHERE id = ?`, fileID,
	).Scan(&owner, &mime, &bound)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("文件不存在")
		}
		return err
	}
	if owner != senderID || bound != "" {
		return fmt.Errorf("文件不可用")
	}
	if kind == "image" && !isImageMIME(mime) {
		return fmt.Errorf("不支持的图片类型")
	}
	return bindFile(tx, fileID, messageID, senderID)
}

func isImageMIME(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func (db *DB) FileVisible(userID, fileID string) (File, error) {
	var f File
	var created int64
	err := db.sql.QueryRow(`
		SELECT f.id, f.owner_id, f.name, f.mime, f.size, f.path, f.message_id, f.created_at
		FROM files f
		JOIN messages m ON m.id = f.message_id
		JOIN members mem ON mem.conversation_id = m.conversation_id AND mem.user_id = ?
		JOIN conversations c ON c.id = m.conversation_id AND c.dissolved = 0
		WHERE f.id = ? AND f.message_id != ''
	`, userID, fileID).Scan(
		&f.ID, &f.OwnerID, &f.Name, &f.Mime, &f.Size, &f.Path, &f.MessageID, &created,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return File{}, fmt.Errorf("无权下载")
		}
		return File{}, err
	}
	f.CreatedAt = unixMilliTime(created)
	return f, nil
}

func (db *DB) SetAvatar(userID, relative string) error {
	return db.withLockRetry(func() error {
		res, err := db.sql.Exec(
			`UPDATE users SET avatar_path = ? WHERE id = ?`,
			relative, userID,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("用户不存在")
		}
		return nil
	})
}

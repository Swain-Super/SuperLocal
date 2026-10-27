package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type User struct {
	ID, Key, Nickname, AvatarPath                       string
	Banned, CanSendMessage, CanSendFile, CanCreateGroup bool
	LastSeenAt, CreatedAt                               time.Time
}

type PublicUser struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
	Online    bool   `json:"online"`
}

type Me struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatarUrl"`
	CanSendMessage bool   `json:"canSendMessage"`
	CanSendFile    bool   `json:"canSendFile"`
	CanCreateGroup bool   `json:"canCreateGroup"`
}

const userColumns = `id, key, nickname, avatar_path, banned, can_send_message, can_send_file, can_create_group, last_seen_at, created_at`

func Public(u User) PublicUser {
	return PublicUser{
		ID:        u.ID,
		Nickname:  u.Nickname,
		AvatarURL: avatarURL(u.ID, u.AvatarPath),
		Online:    false,
	}
}

func MeView(u User) Me {
	return Me{
		ID:             u.ID,
		Key:            u.Key,
		Nickname:       u.Nickname,
		AvatarURL:      avatarURL(u.ID, u.AvatarPath),
		CanSendMessage: u.CanSendMessage,
		CanSendFile:    u.CanSendFile,
		CanCreateGroup: u.CanCreateGroup,
	}
}

func avatarURL(id, path string) string {
	if path == "" {
		return ""
	}
	return "/api/avatars/" + id
}

func (db *DB) LoginOrCreate(key, nickname string, now time.Time) (User, error) {
	existing, err := db.userByKey(key)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return User{}, err
	}
	nick, err := normalizeNickname(nickname)
	if err != nil {
		return User{}, err
	}
	id := NewID(now)
	err = db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`INSERT INTO users (id, key, nickname, banned, can_send_message, can_send_file, can_create_group, created_at)
			 VALUES (?, ?, ?, 0, 1, 1, 1, ?)`,
			id, key, nick, now.UnixMilli(),
		)
		return execErr
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return db.userByKey(key)
		}
		return User{}, err
	}
	return db.UserByID(id)
}

func (db *DB) UserByID(id string) (User, error) {
	return scanUser(db.sql.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// UpdateUserAdmin 写回封禁、发言、发文件和建群四项权限。用户不存在时返回错误。
func (db *DB) UpdateUserAdmin(id string, banned, canSend, canFile, canGroup bool) error {
	return db.withLockRetry(func() error {
		var n int
		err := db.sql.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&n)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("用户不存在")
		}
		_, err = db.sql.Exec(
			`UPDATE users SET banned = ?, can_send_message = ?, can_send_file = ?, can_create_group = ? WHERE id = ?`,
			boolToInt(banned), boolToInt(canSend), boolToInt(canFile), boolToInt(canGroup), id,
		)
		return err
	})
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (db *DB) userByKey(key string) (User, error) {
	return scanUser(db.sql.QueryRow(`SELECT `+userColumns+` FROM users WHERE key = ?`, key))
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var banned, canSendMessage, canSendFile, canCreateGroup int
	var lastSeenAt, createdAt int64
	err := row.Scan(
		&u.ID, &u.Key, &u.Nickname, &u.AvatarPath,
		&banned, &canSendMessage, &canSendFile, &canCreateGroup,
		&lastSeenAt, &createdAt,
	)
	if err != nil {
		return User{}, err
	}
	u.Banned = banned != 0
	u.CanSendMessage = canSendMessage != 0
	u.CanSendFile = canSendFile != 0
	u.CanCreateGroup = canCreateGroup != 0
	u.LastSeenAt = unixMilliTime(lastSeenAt)
	u.CreatedAt = unixMilliTime(createdAt)
	return u, nil
}

func unixMilliTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func normalizeNickname(raw string) (string, error) {
	nick := strings.TrimSpace(raw)
	if strings.Contains(nick, "\n") || strings.Contains(nick, "\r") {
		return "", fmt.Errorf("昵称不能包含换行")
	}
	n := utf8.RuneCountInString(nick)
	if n < 1 || n > 32 {
		return "", fmt.Errorf("昵称长度须为 1–32 个字")
	}
	return nick, nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

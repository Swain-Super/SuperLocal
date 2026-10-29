package store

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// schema 建表。布尔列为 INTEGER 0/1，时间为 Unix 毫秒。
// direct_key 默认空字符串。一对一写入排序后的 userA|userB，群写入自己的 id，避免空字符串挤占唯一约束。
const schema = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT NOT NULL PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    nickname TEXT NOT NULL DEFAULT '',
    avatar_path TEXT NOT NULL DEFAULT '',
    banned INTEGER NOT NULL DEFAULT 0,
    can_send_message INTEGER NOT NULL DEFAULT 1,
    can_send_file INTEGER NOT NULL DEFAULT 1,
    can_create_group INTEGER NOT NULL DEFAULT 1,
    last_seen_at INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0,
    signature TEXT NOT NULL DEFAULT '',
    pendant TEXT NOT NULL DEFAULT '',
    dnd INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT NOT NULL PRIMARY KEY,
    kind TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS admin_user (
    username TEXT NOT NULL PRIMARY KEY,
    password_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
    id TEXT NOT NULL PRIMARY KEY,
    type TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    owner_id TEXT NOT NULL DEFAULT '',
    direct_key TEXT NOT NULL DEFAULT '' UNIQUE,
    dissolved INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS members (
    conversation_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    last_read_id TEXT NOT NULL DEFAULT '',
    joined_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT NOT NULL PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    file_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS files (
    id TEXT NOT NULL PRIMARY KEY,
    owner_id TEXT NOT NULL,
    name TEXT NOT NULL,
    mime TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    path TEXT NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0
);
`

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	db := &DB{sql: sqlDB}
	if err := db.withLockRetry(func() error {
		return migrate(sqlDB)
	}); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

// withLockRetry 在遇到 database is locked 时重试 3 次，间隔 50 毫秒。
func (db *DB) withLockRetry(fn func() error) error {
	err := fn()
	for i := 0; i < 3 && isDatabaseLocked(err); i++ {
		time.Sleep(50 * time.Millisecond)
		err = fn()
	}
	return err
}

func isDatabaseLocked(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

func migrate(sqlDB *sql.DB) error {
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := sqlDB.Exec(stmt); err != nil {
			return err
		}
	}
	extras := []string{
		`ALTER TABLE messages ADD COLUMN reply_to_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN recalled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE members ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE members ADD COLUMN muted INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS reactions (
			message_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			emoji TEXT NOT NULL,
			created_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (message_id, user_id, emoji)
		)`,
		`ALTER TABLE users ADD COLUMN signature TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN pendant TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN dnd INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS friend_requests (
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			created_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (from_id, to_id)
		)`,
		`CREATE TABLE IF NOT EXISTS friends (
			user_id TEXT NOT NULL,
			friend_id TEXT NOT NULL,
			created_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (user_id, friend_id)
		)`,
	}
	for _, stmt := range extras {
		if _, err := sqlDB.Exec(stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return err
		}
	}
	return nil
}

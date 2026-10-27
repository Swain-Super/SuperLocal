package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const sessionTTL = 7 * 24 * time.Hour

func (db *DB) CreateSession(kind, userID string, now time.Time) (string, error) {
	if kind != "user" && kind != "admin" {
		return "", fmt.Errorf("invalid session kind %q", kind)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	expiresAt := now.Add(sessionTTL).UnixMilli()
	err := db.withLockRetry(func() error {
		_, execErr := db.sql.Exec(
			`INSERT INTO sessions (token_hash, kind, user_id, expires_at) VALUES (?, ?, ?, ?)`,
			hash, kind, userID, expiresAt,
		)
		return execErr
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func (db *DB) LookupSession(token string, now time.Time) (kind, userID string, err error) {
	hash := hashToken(token)
	var expiresAt int64
	err = db.sql.QueryRow(
		`SELECT kind, user_id, expires_at FROM sessions WHERE token_hash = ?`, hash,
	).Scan(&kind, &userID, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("session not found")
		}
		return "", "", err
	}
	if now.UnixMilli() >= expiresAt {
		_ = db.DeleteSession(token)
		return "", "", fmt.Errorf("session expired")
	}
	return kind, userID, nil
}

func (db *DB) DeleteSession(token string) error {
	hash := hashToken(token)
	return db.withLockRetry(func() error {
		_, err := db.sql.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hash)
		return err
	})
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

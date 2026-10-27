package store

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestSessions(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)

	user, err := db.LoginOrCreate("user-key-session", "会话用户", now)
	if err != nil {
		t.Fatal(err)
	}

	token, err := db.CreateSession("user", user.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	kind, userID, err := db.LookupSession(token, now)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "user" || userID != user.ID {
		t.Fatalf("kind=%q userID=%q want user/%s", kind, userID, user.ID)
	}

	var plaintextCount int
	if err := db.SQL().QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, token,
	).Scan(&plaintextCount); err != nil {
		t.Fatal(err)
	}
	if plaintextCount != 0 {
		t.Fatalf("plaintext token rows: %d", plaintextCount)
	}

	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	var hashCount int
	if err := db.SQL().QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE token_hash = ?`, hash,
	).Scan(&hashCount); err != nil {
		t.Fatal(err)
	}
	if hashCount != 1 {
		t.Fatalf("sha256 hash rows: %d", hashCount)
	}

	later := now.Add(8 * 24 * time.Hour)
	if _, _, err := db.LookupSession(token, later); err == nil {
		t.Fatal("expected error for expired session")
	}

	token2, err := db.CreateSession("user", user.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteSession(token2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.LookupSession(token2, now); err == nil {
		t.Fatal("expected error after DeleteSession")
	}
}

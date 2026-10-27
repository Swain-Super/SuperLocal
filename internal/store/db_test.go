package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpenCreatesSchema(t *testing.T) {
	db := openTestDB(t)
	var name string
	err := db.SQL().QueryRow(`SELECT name FROM sqlite_master WHERE name = 'users'`).Scan(&name)
	if err != nil || name != "users" {
		t.Fatal(err, name)
	}
}

func TestNewIDOrdersByTime(t *testing.T) {
	a := NewID(time.UnixMilli(1_700_000_000_000))
	b := NewID(time.UnixMilli(1_700_000_000_100))
	if a >= b {
		t.Fatalf("%s >= %s", a, b)
	}
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.SQL().Close()
	})
	return db
}

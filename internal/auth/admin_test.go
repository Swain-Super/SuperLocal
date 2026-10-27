package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"superlocal/internal/store"
)

var adminPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9]{16}$`)

func TestEnsureAdmin(t *testing.T) {
	db := openAuthDB(t)
	dir := t.TempDir()

	username, password, err := EnsureAdmin(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if username != "admin" {
		t.Fatalf("username = %q", username)
	}
	if !adminPasswordPattern.MatchString(password) {
		t.Fatalf("password = %q, want 16 alphanumeric characters", password)
	}

	secretPath := filepath.Join(dir, "admin.secret")
	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want 0600", info.Mode().Perm())
	}
	assertSecret(t, secretPath, username, password)

	if err := CheckAdmin(db, username, password); err != nil {
		t.Fatal(err)
	}
	if err := CheckAdmin(db, username, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}

	if err := os.Remove(secretPath); err != nil {
		t.Fatal(err)
	}
	_, next, err := EnsureAdmin(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	if next == password {
		t.Fatal("expected a new password after secret file was removed")
	}
	if !adminPasswordPattern.MatchString(next) {
		t.Fatalf("reset password = %q, want 16 alphanumeric characters", next)
	}
	assertSecret(t, secretPath, username, next)
	if err := CheckAdmin(db, username, next); err != nil {
		t.Fatal(err)
	}
	if err := CheckAdmin(db, username, password); err == nil {
		t.Fatal("expected old password to fail")
	}
}

func TestChangeAdminPassword(t *testing.T) {
	db := openAuthDB(t)
	dir := t.TempDir()

	username, old, err := EnsureAdmin(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	const next = "new-password-16"
	if err := ChangeAdminPassword(db, dir, next); err != nil {
		t.Fatal(err)
	}
	assertSecret(t, filepath.Join(dir, "admin.secret"), username, next)
	if err := CheckAdmin(db, username, next); err != nil {
		t.Fatal(err)
	}
	if err := CheckAdmin(db, username, old); err == nil {
		t.Fatal("expected old password to fail")
	}
}

func assertSecret(t *testing.T, path, username, password string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 2 || lines[0] != username || lines[1] != password {
		t.Fatalf("secret = %q", body)
	}
}

func openAuthDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.SQL().Close()
	})
	return db
}

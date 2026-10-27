package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"superlocal/internal/store"
)

func TestRetention(t *testing.T) {
	dir := t.TempDir()
	db := openDB(t, dir)
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-49 * time.Hour)
	freshAt := now.Add(-time.Hour)
	boundaryAt := now.Add(-48 * time.Hour)

	jia := login(t, db, "user-key-cleanup-jia", "甲", expiredAt)
	yi := login(t, db, "user-key-cleanup-yi", "乙", expiredAt.Add(time.Millisecond))
	convID, err := db.OpenDirect(jia.ID, yi.ID, expiredAt.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	jiaAvatar := "avatars/" + jia.ID
	yiAvatar := "avatars/" + yi.ID
	writeRel(t, dir, jiaAvatar, "avatar-jia")
	writeRel(t, dir, yiAvatar, "avatar-yi")
	if err := db.SetAvatar(jia.ID, jiaAvatar); err != nil {
		t.Fatal(err)
	}
	if err := db.SetAvatar(yi.ID, yiAvatar); err != nil {
		t.Fatal(err)
	}
	ageRel(t, dir, jiaAvatar, expiredAt)
	ageRel(t, dir, yiAvatar, expiredAt)

	// 过期消息引用的文件未满 48 小时，仍要随消息删除。
	expiredRel := "files/expired-note"
	writeRel(t, dir, expiredRel, "expired-body")
	expiredFileID, err := db.InsertFile(jia.ID, "expired.txt", "text/plain", expiredRel, 12, freshAt)
	if err != nil {
		t.Fatal(err)
	}
	expiredMsg, err := db.PostMessage(jia.ID, convID, "file", "", expiredFileID, expiredAt)
	if err != nil {
		t.Fatal(err)
	}

	// 未过期消息引用的文件已超过 48 小时，文件行和磁盘都保留。
	freshRel := "files/fresh-note"
	writeRel(t, dir, freshRel, "fresh-body")
	freshFileID, err := db.InsertFile(jia.ID, "fresh.txt", "text/plain", freshRel, 10, expiredAt.Add(3*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	freshMsg, err := db.PostMessage(jia.ID, convID, "file", "", freshFileID, freshAt)
	if err != nil {
		t.Fatal(err)
	}

	draftRel := "files/old-draft"
	writeRel(t, dir, draftRel, "draft-body")
	draftID, err := db.InsertFile(jia.ID, "draft.txt", "text/plain", draftRel, 10, expiredAt.Add(4*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	youngDraftRel := "files/young-draft"
	writeRel(t, dir, youngDraftRel, "young-draft")
	youngDraftID, err := db.InsertFile(jia.ID, "young.txt", "text/plain", youngDraftRel, 11, freshAt.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	boundaryMsg, err := db.PostMessage(yi.ID, convID, "text", "刚好 48 小时", "", boundaryAt)
	if err != nil {
		t.Fatal(err)
	}

	// 文字消息的 file_id 为空，文件只通过 message_id 绑在过期消息上。
	oldText, err := db.PostMessage(jia.ID, convID, "text", "过期文字", "", expiredAt.Add(5*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	boundRel := "files/bound-text"
	writeRel(t, dir, boundRel, "bound-body")
	boundID, err := db.InsertFile(jia.ID, "bound.txt", "text/plain", boundRel, 10, freshAt.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.BindFile(boundID, oldText.ID, jia.ID); err != nil {
		t.Fatal(err)
	}

	if err := Run(db, dir, now); err != nil {
		t.Fatal(err)
	}

	if count(t, db, `SELECT COUNT(*) FROM messages WHERE id = ?`, expiredMsg.ID) != 0 {
		t.Fatal("expired message remains")
	}
	if count(t, db, `SELECT COUNT(*) FROM files WHERE id = ?`, expiredFileID) != 0 {
		t.Fatal("file referenced by expired message remains")
	}
	if count(t, db, `SELECT COUNT(*) FROM files WHERE id = ?`, draftID) != 0 {
		t.Fatal("expired unbound file remains")
	}
	if count(t, db, `SELECT COUNT(*) FROM messages WHERE id = ?`, oldText.ID) != 0 {
		t.Fatal("expired text message remains")
	}
	if count(t, db, `SELECT COUNT(*) FROM files WHERE id = ?`, boundID) != 0 {
		t.Fatal("file bound to expired message remains")
	}
	missingFile(t, filepath.Join(dir, filepath.FromSlash(expiredRel)))
	missingFile(t, filepath.Join(dir, filepath.FromSlash(draftRel)))
	missingFile(t, filepath.Join(dir, filepath.FromSlash(boundRel)))

	if count(t, db, `SELECT COUNT(*) FROM messages WHERE id = ?`, freshMsg.ID) != 1 {
		t.Fatal("fresh message was deleted")
	}
	if count(t, db, `SELECT COUNT(*) FROM messages WHERE id = ?`, boundaryMsg.ID) != 1 {
		t.Fatal("message at exactly 48 hours was deleted")
	}
	if got := fileMessageID(t, db, freshFileID); got != freshMsg.ID {
		t.Fatalf("fresh file message_id = %q, want %s", got, freshMsg.ID)
	}
	if body := readRel(t, dir, freshRel); body != "fresh-body" {
		t.Fatalf("fresh file body = %q", body)
	}
	if count(t, db, `SELECT COUNT(*) FROM files WHERE id = ?`, youngDraftID) != 1 {
		t.Fatal("unbound file younger than 48 hours was deleted")
	}
	if fileMessageID(t, db, youngDraftID) != "" {
		t.Fatal("young draft message_id changed")
	}
	if body := readRel(t, dir, youngDraftRel); body != "young-draft" {
		t.Fatalf("young draft body = %q", body)
	}

	if count(t, db, `SELECT COUNT(*) FROM users`) != 2 {
		t.Fatal("users were deleted")
	}
	for _, want := range []struct {
		id, nick, avatar, body string
	}{
		{jia.ID, "甲", jiaAvatar, "avatar-jia"},
		{yi.ID, "乙", yiAvatar, "avatar-yi"},
	} {
		user, err := db.UserByID(want.id)
		if err != nil {
			t.Fatal(err)
		}
		if user.Nickname != want.nick || user.AvatarPath != want.avatar {
			t.Fatalf("user %+v", user)
		}
		if body := readRel(t, dir, want.avatar); body != want.body {
			t.Fatalf("avatar %s body = %q", want.avatar, body)
		}
	}
	if count(t, db, `SELECT COUNT(*) FROM members`) != 2 {
		t.Fatal("memberships were deleted")
	}
	if count(t, db, `SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`, convID, jia.ID) != 1 {
		t.Fatal("jia membership missing")
	}
	if count(t, db, `SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`, convID, yi.ID) != 1 {
		t.Fatal("yi membership missing")
	}
	if count(t, db, `SELECT COUNT(*) FROM conversations WHERE id = ?`, convID) != 1 {
		t.Fatal("conversation was deleted")
	}
}

func openDB(t *testing.T, dir string) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.SQL().Close()
	})
	return db
}

func login(t *testing.T, db *store.DB, key, nickname string, now time.Time) store.User {
	t.Helper()
	user, err := db.LoginOrCreate(key, nickname, now)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func ageRel(t *testing.T, dir, rel string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(dir, filepath.FromSlash(rel)), at, at); err != nil {
		t.Fatal(err)
	}
}

func writeRel(t *testing.T, dir, rel, body string) {
	t.Helper()
	dest := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readRel(t *testing.T, dir, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func missingFile(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("file still exists: %s", path)
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func count(t *testing.T, db *store.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.SQL().QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func fileMessageID(t *testing.T, db *store.DB, fileID string) string {
	t.Helper()
	var messageID string
	if err := db.SQL().QueryRow(`SELECT message_id FROM files WHERE id = ?`, fileID).Scan(&messageID); err != nil {
		t.Fatal(err)
	}
	return messageID
}

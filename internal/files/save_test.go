package files

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"superlocal/internal/store"
)

func TestSaveUpload(t *testing.T) {
	dir := t.TempDir()
	ownerID := "owner-save-upload"
	const maxSize = int64(4)

	_, _, err := SaveUpload(dir, ownerID, "notes.txt", "text/plain", bytes.NewReader([]byte("12345")), maxSize, false)
	if err == nil {
		t.Fatal("expected error when file exceeds max")
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Fatalf("leftover files after oversize upload: %v", files)
	}

	_, _, err = SaveUpload(dir, ownerID, "fake.png", "text/plain", bytes.NewReader([]byte("nope")), 1024, true)
	if err == nil {
		t.Fatal("expected error for text/plain when imageOnly")
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Fatalf("leftover files after rejected image: %v", files)
	}

	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	rel, size, err := SaveUpload(dir, ownerID, "pic.png", "image/png", bytes.NewReader(png), 1024, true)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(png)) {
		t.Fatalf("size = %d", size)
	}
	if !strings.HasPrefix(rel, "files/") || strings.Contains(rel, `\`) || strings.Contains(rel, "..") || strings.Contains(rel, ownerID) || strings.Contains(rel, "pic.png") {
		t.Fatalf("path = %q", rel)
	}
	got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("content = %q", got)
	}
	if under(t, dir, "tmp") != 0 {
		t.Fatal("temp file remained after successful upload")
	}

	exact, exactSize, err := SaveUpload(dir, ownerID, "four.txt", "text/plain", bytes.NewReader([]byte("1234")), maxSize, false)
	if err != nil {
		t.Fatal(err)
	}
	if exactSize != maxSize || !strings.HasPrefix(exact, "files/") {
		t.Fatalf("exact path %q size %d", exact, exactSize)
	}

	before := regularFiles(t, dir)
	_, _, err = SaveUpload(dir, ownerID, "big.bin", "application/octet-stream", bytes.NewReader([]byte("12345")), maxSize, false)
	if err == nil {
		t.Fatal("expected error when file exceeds max")
	}
	after := regularFiles(t, dir)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("files changed after failed upload\nbefore %v\nafter %v", before, after)
	}
}

func TestVisibility(t *testing.T) {
	db := openDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia := login(t, db, "user-key-file-jia", "甲", now)
	yi := login(t, db, "user-key-file-yi", "乙", now.Add(time.Millisecond))
	bing := login(t, db, "user-key-file-bing", "丙", now.Add(2*time.Millisecond))
	convID, err := db.CreateGroup(jia.ID, "文件群", []string{yi.ID, bing.ID}, now)
	if err != nil {
		t.Fatal(err)
	}

	unboundID, err := db.InsertFile(jia.ID, "draft.txt", "text/plain", "files/draft", 4, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.FileVisible(yi.ID, unboundID); err == nil {
		t.Fatal("unbound file visible to another user")
	}
	if _, err := db.FileVisible(jia.ID, unboundID); err == nil {
		t.Fatal("unbound file visible before it is sent")
	}

	textMsg, err := db.PostMessage(jia.ID, convID, "text", "先有一条消息", "", now.Add(3*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	sharedID, err := db.InsertFile(jia.ID, "notes.txt", "text/plain", "files/notes", 5, now.Add(4*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.BindFile(sharedID, textMsg.ID, yi.ID); err == nil {
		t.Fatal("expected error when binding another user's file")
	}
	if got := fileMessageID(t, db, sharedID); got != "" {
		t.Fatalf("message_id = %q after rejected bind", got)
	}
	if err := db.BindFile(sharedID, textMsg.ID, jia.ID); err != nil {
		t.Fatal(err)
	}
	if got := fileMessageID(t, db, sharedID); got != textMsg.ID {
		t.Fatalf("message_id = %q, want %s", got, textMsg.ID)
	}

	got, err := db.FileVisible(yi.ID, sharedID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sharedID || got.OwnerID != jia.ID || got.Name != "notes.txt" || got.Mime != "text/plain" || got.Path != "files/notes" || got.MessageID != textMsg.ID || got.Size != 5 || got.CreatedAt.UnixMilli() != now.Add(4*time.Millisecond).UnixMilli() {
		t.Fatalf("%+v", got)
	}
	if _, err := db.FileVisible(jia.ID, sharedID); err != nil {
		t.Fatal(err)
	}

	if err := db.Leave(bing.ID, convID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FileVisible(bing.ID, sharedID); err == nil {
		t.Fatal("left member can still see the file")
	}
	if _, err := db.FileVisible(yi.ID, sharedID); err != nil {
		t.Fatal(err)
	}
	if err := db.BindFile(sharedID, textMsg.ID, jia.ID); err == nil {
		t.Fatal("expected error when binding a file twice")
	}

	messagesBefore := countMessages(t, db)
	foreignID, err := db.InsertFile(yi.ID, "secret.txt", "text/plain", "files/secret", 3, now.Add(5*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(jia.ID, convID, "file", "secret.txt", foreignID, now.Add(6*time.Millisecond)); err == nil {
		t.Fatal("expected error when sending another user's file")
	}
	if countMessages(t, db) != messagesBefore {
		t.Fatalf("messages = %d, want %d", countMessages(t, db), messagesBefore)
	}
	if got := fileMessageID(t, db, foreignID); got != "" {
		t.Fatalf("foreign message_id = %q", got)
	}

	for _, mime := range []string{"image/jpeg", "image/png", "image/webp", "image/gif"} {
		id, err := db.InsertFile(jia.ID, "pic", mime, "files/"+mime, 8, now.Add(7*time.Millisecond))
		if err != nil {
			t.Fatal(err)
		}
		msg, err := db.PostMessage(jia.ID, convID, "image", "", id, now.Add(8*time.Millisecond))
		if err != nil {
			t.Fatalf("mime %s: %v", mime, err)
		}
		if msg.Kind != "image" || msg.FileID != id {
			t.Fatalf("%+v", msg)
		}
		if got := fileMessageID(t, db, id); got != msg.ID {
			t.Fatalf("mime %s message_id = %q, want %s", mime, got, msg.ID)
		}
		visible, err := db.FileVisible(yi.ID, id)
		if err != nil {
			t.Fatalf("mime %s: %v", mime, err)
		}
		if visible.Mime != mime || visible.MessageID != msg.ID {
			t.Fatalf("%+v", visible)
		}
	}

	badID, err := db.InsertFile(jia.ID, "note.txt", "text/plain", "files/note", 4, now.Add(9*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	messagesBefore = countMessages(t, db)
	if _, err := db.PostMessage(jia.ID, convID, "image", "", badID, now.Add(10*time.Millisecond)); err == nil {
		t.Fatal("expected error for image mime text/plain")
	}
	if countMessages(t, db) != messagesBefore {
		t.Fatalf("messages = %d, want %d", countMessages(t, db), messagesBefore)
	}
	if got := fileMessageID(t, db, badID); got != "" {
		t.Fatalf("rejected image message_id = %q", got)
	}

	fileMsg, err := db.PostMessage(jia.ID, convID, "file", "note.txt", badID, now.Add(11*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if fileMsg.Kind != "file" || fileMsg.FileID != badID || fileMsg.Body != "note.txt" {
		t.Fatalf("%+v", fileMsg)
	}
	if got := fileMessageID(t, db, badID); got != fileMsg.ID {
		t.Fatalf("file message_id = %q, want %s", got, fileMsg.ID)
	}
	if _, err := db.PostMessage(jia.ID, convID, "file", "again.txt", badID, now.Add(12*time.Millisecond)); err == nil {
		t.Fatal("expected error when sending a file that is already bound")
	}
}

func TestAvatar(t *testing.T) {
	dir := t.TempDir()
	db := openDB(t)
	now := time.Unix(1_700_000_000, 0)
	user := login(t, db, "user-key-avatar", "甲", now)
	const avatarMax = 2 * 1024 * 1024

	_, err := SaveAvatar(dir, user.ID, "text/plain", bytes.NewReader([]byte("nope")))
	if err == nil {
		t.Fatal("expected error for text/plain avatar")
	}
	_, err = SaveAvatar(dir, user.ID, "image/bmp", bytes.NewReader([]byte("BM")))
	if err == nil {
		t.Fatal("expected error for image/bmp avatar")
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Fatalf("leftover files after rejected avatar: %v", files)
	}

	_, err = SaveAvatar(dir, user.ID, "image/png", io.LimitReader(repeatReader('a'), avatarMax+1))
	if err == nil {
		t.Fatal("expected error when avatar exceeds 2MB")
	}
	if files := regularFiles(t, dir); len(files) != 0 {
		t.Fatalf("leftover files after oversize avatar: %v", files)
	}

	rel, err := SaveAvatar(dir, user.ID, "image/png", io.LimitReader(repeatReader('b'), avatarMax))
	if err != nil {
		t.Fatal(err)
	}
	if rel != "avatars/"+user.ID {
		t.Fatalf("path = %q", rel)
	}
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != avatarMax {
		t.Fatalf("size = %d", info.Size())
	}

	_, err = SaveAvatar(dir, user.ID, "image/png", io.LimitReader(repeatReader('c'), avatarMax+1))
	if err == nil {
		t.Fatal("expected error when replacing avatar with an oversize file")
	}
	kept, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(kept)) != avatarMax || kept[0] != 'b' {
		t.Fatal("oversize avatar replaced the existing file")
	}

	var lastRel string
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp", "image/gif"} {
		body := []byte("avatar-" + mime)
		lastRel, err = SaveAvatar(dir, user.ID, mime, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("mime %s: %v", mime, err)
		}
		if lastRel != "avatars/"+user.ID {
			t.Fatalf("mime %s path = %q", mime, lastRel)
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(lastRel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, body) {
			t.Fatalf("mime %s content = %q", mime, got)
		}
	}
	if under(t, dir, "tmp") != 0 {
		t.Fatal("temp file remained after avatar upload")
	}
	if err := db.SetAvatar(user.ID, lastRel); err != nil {
		t.Fatal(err)
	}
	if err := db.SetAvatar(user.ID, lastRel); err != nil {
		t.Fatal(err)
	}
	stored, err := db.UserByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AvatarPath != lastRel {
		t.Fatalf("avatar_path = %q", stored.AvatarPath)
	}
	if err := db.SetAvatar("missing-user", lastRel); err == nil {
		t.Fatal("expected error when setting an avatar for a missing user")
	}
}

func openDB(t *testing.T) *store.DB {
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

func login(t *testing.T, db *store.DB, key, nickname string, now time.Time) store.User {
	t.Helper()
	user, err := db.LoginOrCreate(key, nickname, now)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func fileMessageID(t *testing.T, db *store.DB, fileID string) string {
	t.Helper()
	var messageID string
	if err := db.SQL().QueryRow(`SELECT message_id FROM files WHERE id = ?`, fileID).Scan(&messageID); err != nil {
		t.Fatal(err)
	}
	return messageID
}

func countMessages(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func regularFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func under(t *testing.T, dir, name string) int {
	t.Helper()
	n := 0
	root := filepath.Join(dir, name)
	for _, path := range regularFiles(t, dir) {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			n++
		}
	}
	return n
}

type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}

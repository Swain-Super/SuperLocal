package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDirect(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	first, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.OpenDirect(yi.ID, jia.ID, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("conversation id = %q / %q", first, second)
	}
	third, err := db.OpenDirect(jia.ID, yi.ID, now.Add(2*time.Second))
	if err != nil || third != first {
		t.Fatalf("id %q err %v", third, err)
	}

	var convCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&convCount); err != nil {
		t.Fatal(err)
	}
	if convCount != 1 {
		t.Fatalf("conversations = %d", convCount)
	}
	wantKey := jia.ID + "|" + yi.ID
	if jia.ID > yi.ID {
		wantKey = yi.ID + "|" + jia.ID
	}
	var gotKey string
	if err := db.SQL().QueryRow(`SELECT direct_key FROM conversations WHERE id = ?`, first).Scan(&gotKey); err != nil {
		t.Fatal(err)
	}
	if gotKey != wantKey {
		t.Fatalf("direct_key = %q, want %q", gotKey, wantKey)
	}
	var memberCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, first).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 2 {
		t.Fatalf("members = %d", memberCount)
	}

	for _, id := range []string{jia.ID, yi.ID} {
		list, err := db.ListConversations(id, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("user %s list before message: %+v", id, list)
		}
	}

	msg, err := db.PostMessage(jia.ID, first, "text", "你好", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ConversationID != first || msg.SenderID != jia.ID || msg.SenderNickname != "甲" || msg.Kind != "text" || msg.Body != "你好" || msg.FileID != "" || msg.ID == "" {
		t.Fatalf("%+v", msg)
	}
	if msg.CreatedAt.UnixMilli() != now.UnixMilli() {
		t.Fatalf("createdAt %s", msg.CreatedAt)
	}

	listJia, err := db.ListConversations(jia.ID, map[string]bool{yi.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(listJia) != 1 {
		t.Fatalf("jia list: %+v", listJia)
	}
	view := listJia[0]
	if view.ID != first || view.Type != "direct" || view.Title != "乙" || view.MemberCount != 2 || view.Unread != 0 || view.LastBody != "你好" || view.LastAt.UnixMilli() != now.UnixMilli() {
		t.Fatalf("%+v", view)
	}
	if view.Peer == nil || view.Peer.ID != yi.ID || view.Peer.Nickname != "乙" || !view.Peer.Online {
		t.Fatalf("%+v", view.Peer)
	}
	rawJia, err := json.Marshal(listJia)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawJia), "user-key-yi") || strings.Contains(string(rawJia), `"key"`) {
		t.Fatalf("json leaked key: %s", rawJia)
	}

	listYi, err := db.ListConversations(yi.ID, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listYi) != 1 || listYi[0].Title != "甲" || listYi[0].Unread != 1 || listYi[0].Peer == nil || listYi[0].Peer.Online || listYi[0].Peer.Nickname != "甲" {
		t.Fatalf("%+v", listYi)
	}
	rawYi, err := json.Marshal(listYi)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rawYi), "user-key-jia") || strings.Contains(string(rawYi), `"key"`) {
		t.Fatalf("json leaked key: %s", rawYi)
	}

	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&convCount); err != nil {
		t.Fatal(err)
	}
	if convCount != 1 {
		t.Fatalf("conversations after send = %d", convCount)
	}

	if _, err := db.SQL().Exec(`UPDATE users SET banned = 1 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(jia.ID, first, "text", "封禁后", "", now); err == nil {
		t.Fatal("expected error for banned sender")
	}
	if _, err := db.SQL().Exec(`UPDATE users SET banned = 0 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := db.SQL().Exec(`UPDATE users SET can_send_message = 0 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(jia.ID, first, "text", "禁言后", "", now); err == nil {
		t.Fatal("expected error when can_send_message is 0")
	}
	if _, err := db.SQL().Exec(`UPDATE users SET can_send_message = 1 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := db.SQL().Exec(`UPDATE users SET banned = 1 WHERE id = ?`, yi.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(jia.ID, first, "text", "对方被封", "", now); err == nil {
		t.Fatal("expected error when the other member is banned")
	}

	var messageCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("messages = %d, rejected sends were stored", messageCount)
	}
}

func TestTextLimit(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-limit-a", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-limit-b", "乙", now)
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}

	okBody := strings.Repeat("字", 4000)
	msg, err := db.PostMessage(jia.ID, convID, "text", okBody, "", now)
	if err != nil || msg.Body != okBody {
		t.Fatalf("%+v %v", msg, err)
	}
	if _, err := db.PostMessage(jia.ID, convID, "text", strings.Repeat("字", 4001), "", now); err == nil {
		t.Fatal("expected error for text over 4000 runes")
	}
	for _, body := range []string{"", "   ", "\n\t"} {
		if _, err := db.PostMessage(jia.ID, convID, "text", body, "", now); err == nil {
			t.Fatalf("expected error for blank body %q", body)
		}
	}

	var messageCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("messages = %d", messageCount)
	}
}

func TestNonMember(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-mem-a", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-mem-b", "乙", now)
	if err != nil {
		t.Fatal(err)
	}
	bing, err := db.LoginOrCreate("user-key-mem-c", "丙", now)
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PostMessage(bing.ID, convID, "text", "闯入", "", now); err == nil {
		t.Fatal("expected error for non-member")
	}
	if _, err := db.PostMessage(jia.ID, "missing-conversation", "text", "不存在", "", now); err == nil {
		t.Fatal("expected error for missing conversation")
	}

	var messageCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("messages = %d", messageCount)
	}
	msg, err := db.PostMessage(yi.ID, convID, "text", "成员可以发", "", now)
	if err != nil || msg.Body != "成员可以发" || msg.SenderNickname != "乙" {
		t.Fatalf("%+v %v", msg, err)
	}
}

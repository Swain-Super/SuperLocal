package store

import (
	"encoding/json"
	"fmt"
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
	if view.ID != first || view.Type != "direct" || view.OwnerID != "" || view.Title != "乙" || view.MemberCount != 2 || view.Unread != 0 || view.LastBody != "你好" || view.LastAt.UnixMilli() != now.UnixMilli() {
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

func TestGroup(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-group-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-group-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	bing, err := db.LoginOrCreate("user-key-group-bing", "丙", now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ding, err := db.LoginOrCreate("user-key-group-ding", "丁", now.Add(3*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	wu, err := db.LoginOrCreate("user-key-group-wu", "戊", now.Add(4*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL().Exec(`UPDATE users SET banned = 1 WHERE id = ?`, ding.ID); err != nil {
		t.Fatal(err)
	}

	name41 := strings.Repeat("名", 41)
	rejects := []struct {
		owner string
		name  string
		ids   []string
	}{
		{jia.ID, "", []string{yi.ID}},
		{jia.ID, "   ", []string{yi.ID}},
		{jia.ID, name41, []string{yi.ID}},
		{jia.ID, "群", nil},
		{jia.ID, "群", []string{}},
		{jia.ID, "群", []string{jia.ID}},
		{jia.ID, "群", []string{ding.ID}},
		{jia.ID, "群", []string{yi.ID, ding.ID}},
		{jia.ID, "群", []string{"missing-user"}},
	}
	for _, c := range rejects {
		if _, err := db.CreateGroup(c.owner, c.name, c.ids, now); err == nil {
			t.Fatalf("expected error for name %q members %v", c.name, c.ids)
		}
	}
	if _, err := db.SQL().Exec(`UPDATE users SET can_create_group = 0 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateGroup(jia.ID, "群", []string{yi.ID}, now); err == nil {
		t.Fatal("expected error when CanCreateGroup is false")
	}
	if _, err := db.SQL().Exec(`UPDATE users SET can_create_group = 1 WHERE id = ?`, jia.ID); err != nil {
		t.Fatal(err)
	}
	var convCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&convCount); err != nil {
		t.Fatal(err)
	}
	if convCount != 0 {
		t.Fatalf("rejected creates stored %d conversations", convCount)
	}

	name := strings.Repeat("组", 40)
	id, err := db.CreateGroup(jia.ID, "  "+name+"  ", []string{yi.ID, "", yi.ID, jia.ID}, now)
	if err != nil || id == "" {
		t.Fatalf("id %q err %v", id, err)
	}
	var gotType, gotName, ownerID, directKey string
	var dissolved int
	if err := db.SQL().QueryRow(
		`SELECT type, name, owner_id, direct_key, dissolved FROM conversations WHERE id = ?`, id,
	).Scan(&gotType, &gotName, &ownerID, &directKey, &dissolved); err != nil {
		t.Fatal(err)
	}
	if gotType != "group" || gotName != name || ownerID != jia.ID || directKey != id || dissolved != 0 {
		t.Fatalf("type %q name %q owner %q direct_key %q dissolved %d", gotType, gotName, ownerID, directKey, dissolved)
	}
	var memberCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, id).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 2 {
		t.Fatalf("members = %d", memberCount)
	}

	second, err := db.CreateGroup(jia.ID, "第二群", []string{bing.ID}, now.Add(time.Second))
	if err != nil || second == "" || second == id {
		t.Fatalf("second %q err %v", second, err)
	}
	var secondKey string
	if err := db.SQL().QueryRow(`SELECT direct_key FROM conversations WHERE id = ?`, second).Scan(&secondKey); err != nil {
		t.Fatal(err)
	}
	if secondKey != second {
		t.Fatalf("second direct_key = %q", secondKey)
	}
	if err := db.Dissolve(second); err != nil {
		t.Fatal(err)
	}
	var secondMembers int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, second).Scan(&secondMembers); err != nil {
		t.Fatal(err)
	}
	if secondMembers != 0 {
		t.Fatalf("dissolved empty group members = %d", secondMembers)
	}

	listJia, err := db.ListConversations(jia.ID, map[string]bool{yi.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(listJia) != 1 {
		t.Fatalf("jia list before messages: %+v", listJia)
	}
	view := listJia[0]
	if view.ID != id || view.Type != "group" || view.OwnerID != jia.ID || view.Title != name || view.MemberCount != 2 || view.Unread != 0 || view.LastBody != "" || !view.LastAt.IsZero() || view.Peer != nil {
		t.Fatalf("%+v", view)
	}
	groupRaw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(groupRaw), `"ownerId":"`+jia.ID+`"`) {
		t.Fatalf("group conversation JSON missing ownerId: %s", groupRaw)
	}
	if strings.Contains(string(groupRaw), "user-key-group-") || strings.Contains(string(groupRaw), `"key"`) {
		t.Fatalf("group conversation JSON leaked key: %s", groupRaw)
	}
	listYi, err := db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listYi) != 1 || listYi[0].ID != id || listYi[0].Title != name || listYi[0].Type != "group" || listYi[0].Peer != nil {
		t.Fatalf("%+v", listYi)
	}
	listBing, err := db.ListConversations(bing.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listBing) != 0 {
		t.Fatalf("bing list before join: %+v", listBing)
	}

	if err := db.AddMembers(yi.ID, id, []string{wu.ID}, now); err == nil {
		t.Fatal("expected error when non-owner adds members")
	}
	if err := db.AddMembers(jia.ID, id, []string{ding.ID}, now); err == nil {
		t.Fatal("expected error when adding a banned user")
	}
	if err := db.AddMembers(jia.ID, id, []string{wu.ID, ding.ID}, now); err == nil {
		t.Fatal("expected error when a new member is paired with a banned user")
	}
	var wuCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`, id, wu.ID).Scan(&wuCount); err != nil {
		t.Fatal(err)
	}
	if wuCount != 0 {
		t.Fatal("rejected add stored a member")
	}
	if err := db.AddMembers(jia.ID, id, []string{bing.ID, bing.ID}, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := db.AddMembers(jia.ID, id, []string{bing.ID}, now); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, id).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 3 {
		t.Fatalf("members after add = %d", memberCount)
	}
	listBing, err = db.ListConversations(bing.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listBing) != 1 || listBing[0].ID != id || listBing[0].MemberCount != 3 || listBing[0].Title != name {
		t.Fatalf("%+v", listBing)
	}
	fromBing, err := db.PostMessage(bing.ID, id, "text", "丙来了", "", now.Add(3*time.Second))
	if err != nil || fromBing.Body != "丙来了" || fromBing.SenderNickname != "丙" {
		t.Fatalf("%+v %v", fromBing, err)
	}

	if err := db.Leave(yi.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := db.Leave(yi.ID, id); err == nil {
		t.Fatal("expected error when leaving twice")
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listYi) != 0 {
		t.Fatalf("yi list after leave: %+v", listYi)
	}
	if _, err := db.PostMessage(yi.ID, id, "text", "已退出", "", now); err == nil {
		t.Fatal("expected error when a former member posts")
	}
	if _, err := db.Messages(yi.ID, id, ""); err == nil {
		t.Fatal("expected error when a former member reads messages")
	}
	listJia, err = db.ListConversations(jia.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listJia) != 1 || listJia[0].ID != id || listJia[0].MemberCount != 2 {
		t.Fatalf("jia list after yi left: %+v", listJia)
	}

	fromJia, err := db.PostMessage(jia.ID, id, "text", "大家好", "", now.Add(4*time.Second))
	if err != nil || fromJia.ConversationID != id {
		t.Fatalf("%+v %v", fromJia, err)
	}
	if err := db.Leave(jia.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := db.AddMembers(jia.ID, id, []string{wu.ID}, now); err == nil {
		t.Fatal("expected error when the owner has left")
	}
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`, id, wu.ID).Scan(&wuCount); err != nil {
		t.Fatal(err)
	}
	if wuCount != 0 {
		t.Fatal("owner who left added a member")
	}
	listJia, err = db.ListConversations(jia.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listJia) != 0 {
		t.Fatalf("jia list after leave: %+v", listJia)
	}
	listBing, err = db.ListConversations(bing.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listBing) != 1 || listBing[0].MemberCount != 1 {
		t.Fatalf("bing list after owner left: %+v", listBing)
	}

	if err := db.Dissolve(id); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, id).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 0 {
		t.Fatalf("members after dissolve = %d", memberCount)
	}
	var messageCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, id).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 2 {
		t.Fatalf("messages after dissolve = %d", messageCount)
	}
	var dissolvedFlag int
	if err := db.SQL().QueryRow(`SELECT dissolved FROM conversations WHERE id = ?`, id).Scan(&dissolvedFlag); err != nil {
		t.Fatal(err)
	}
	if dissolvedFlag != 1 {
		t.Fatalf("dissolved = %d", dissolvedFlag)
	}
	listBing, err = db.ListConversations(bing.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listBing) != 0 {
		t.Fatalf("bing list after dissolve: %+v", listBing)
	}
	if _, err := db.PostMessage(bing.ID, id, "text", "已解散", "", now); err == nil {
		t.Fatal("expected error when posting to a dissolved group")
	}
}

func TestUnread(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_100, 0)
	jia, err := db.LoginOrCreate("user-key-unread-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-unread-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	bing, err := db.LoginOrCreate("user-key-unread-bing", "丙", now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.CreateGroup(jia.ID, "未读群", []string{yi.ID}, now)
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := db.CreateGroup(jia.ID, "另一群", []string{bing.ID}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}

	msg1, err := db.PostMessage(jia.ID, convID, "text", "一", "", now)
	if err != nil {
		t.Fatal(err)
	}
	listYi, err := db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listYi) != 1 || listYi[0].Unread != 1 {
		t.Fatalf("yi unread after first message: %+v", listYi)
	}
	listJia, err := db.ListConversations(jia.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	jiaUnread, foundJia := unreadOf(listJia, convID)
	if !foundJia || jiaUnread != 0 {
		t.Fatalf("jia unread = %d found %v list %+v", jiaUnread, foundJia, listJia)
	}

	got, err := db.Messages(yi.ID, convID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != msg1.ID || got[0].Body != "一" || got[0].SenderNickname != "甲" || got[0].SenderID != jia.ID {
		t.Fatalf("%+v", got)
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listYi[0].Unread != 1 {
		t.Fatalf("Messages changed unread: %+v", listYi[0])
	}
	if got, err = db.Messages(yi.ID, convID, msg1.ID); err != nil || len(got) != 0 {
		t.Fatalf("%+v %v", got, err)
	}

	msg2, err := db.PostMessage(jia.ID, convID, "text", "二", "", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	got, err = db.Messages(yi.ID, convID, msg1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != msg2.ID || got[0].Body != "二" {
		t.Fatalf("%+v", got)
	}
	var readID string
	if err := db.SQL().QueryRow(`SELECT last_read_id FROM members WHERE conversation_id = ? AND user_id = ?`, convID, yi.ID).Scan(&readID); err != nil {
		t.Fatal(err)
	}
	if readID != "" {
		t.Fatalf("last_read_id = %q after Messages", readID)
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listYi[0].Unread != 2 {
		t.Fatalf("yi unread = %d", listYi[0].Unread)
	}

	other, err := db.PostMessage(jia.ID, otherID, "text", "别处", "", now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkRead(yi.ID, convID, other.ID); err == nil {
		t.Fatal("expected error when marking a message from another conversation")
	}
	if err := db.MarkRead(bing.ID, convID, msg2.ID); err == nil {
		t.Fatal("expected error when a non-member marks read")
	}
	if err := db.SQL().QueryRow(`SELECT last_read_id FROM members WHERE conversation_id = ? AND user_id = ?`, convID, yi.ID).Scan(&readID); err != nil {
		t.Fatal(err)
	}
	if readID != "" {
		t.Fatalf("last_read_id changed to %q", readID)
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listYi[0].Unread != 2 {
		t.Fatalf("unread changed by rejected MarkRead: %d", listYi[0].Unread)
	}

	if err := db.MarkRead(yi.ID, convID, msg2.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.SQL().QueryRow(`SELECT last_read_id FROM members WHERE conversation_id = ? AND user_id = ?`, convID, yi.ID).Scan(&readID); err != nil {
		t.Fatal(err)
	}
	if readID != msg2.ID {
		t.Fatalf("last_read_id = %q", readID)
	}
	var ownerRead string
	if err := db.SQL().QueryRow(`SELECT last_read_id FROM members WHERE conversation_id = ? AND user_id = ?`, convID, jia.ID).Scan(&ownerRead); err != nil {
		t.Fatal(err)
	}
	if ownerRead != "" {
		t.Fatalf("owner last_read_id = %q", ownerRead)
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listYi[0].Unread != 0 {
		t.Fatalf("yi unread after MarkRead = %d", listYi[0].Unread)
	}
	listJia, err = db.ListConversations(jia.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	jiaUnread, foundJia = unreadOf(listJia, convID)
	if !foundJia || jiaUnread != 0 {
		t.Fatalf("jia unread after yi MarkRead = %d found %v", jiaUnread, foundJia)
	}

	got, err = db.Messages(yi.ID, convID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != msg1.ID || got[1].ID != msg2.ID {
		t.Fatalf("%+v", got)
	}
	if got[1].ID <= got[0].ID {
		t.Fatalf("messages not ordered by id: %s %s", got[0].ID, got[1].ID)
	}
	listYi, err = db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if listYi[0].Unread != 0 {
		t.Fatalf("Messages after MarkRead changed unread: %d", listYi[0].Unread)
	}
	if _, err := db.Messages(bing.ID, convID, ""); err == nil {
		t.Fatal("expected error when a non-member reads messages")
	}
}

func TestSearchUsers(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_200, 0)
	banned, err := db.LoginOrCreate("user-key-search-banned", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL().Exec(`UPDATE users SET banned = 1 WHERE id = ?`, banned.ID); err != nil {
		t.Fatal(err)
	}
	match, err := db.LoginOrCreate("user-key-search-match", "甲乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-search-yi", "乙", now.Add(2*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.SearchUsers("甲")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != match.ID || got[0].Nickname != "甲乙" {
		t.Fatalf("before limit: %+v", got)
	}

	keys := []string{match.Key, banned.Key, yi.Key}
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("user-key-search-%02d", i)
		nick := fmt.Sprintf("甲%02d", i)
		if _, err := db.LoginOrCreate(key, nick, now.Add(time.Duration(i+1)*time.Millisecond)); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}

	got, err = db.SearchUsers("甲")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 50 {
		t.Fatalf("len = %d", len(got))
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, `"key"`) {
		t.Fatalf("json leaked key: %s", text)
	}
	for _, key := range keys {
		if strings.Contains(text, key) {
			t.Fatalf("json leaked key %s", key)
		}
	}
	for _, u := range got {
		if u.ID == banned.ID || u.Nickname == "甲" || u.ID == yi.ID || u.Nickname == "乙" {
			t.Fatalf("unexpected user %+v", u)
		}
		if !strings.Contains(u.Nickname, "甲") {
			t.Fatalf("nickname %q", u.Nickname)
		}
		if u.Online {
			t.Fatalf("%+v", u)
		}
	}
}

func TestLeaveDirect(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_300, 0)
	jia, err := db.LoginOrCreate("user-key-leave-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-leave-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Leave(jia.ID, convID); err == nil {
		t.Fatal("expected error when leaving a direct conversation")
	}
	var memberCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM members WHERE conversation_id = ?`, convID).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 2 {
		t.Fatalf("members = %d", memberCount)
	}
	for _, userID := range []string{jia.ID, yi.ID} {
		var n int
		if err := db.SQL().QueryRow(
			`SELECT COUNT(*) FROM members WHERE conversation_id = ? AND user_id = ?`,
			convID, userID,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("user %s membership = %d", userID, n)
		}
	}
}

func unreadOf(list []ConversationView, id string) (int, bool) {
	for _, view := range list {
		if view.ID == id {
			return view.Unread, true
		}
	}
	return 0, false
}

func TestPermissions(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-perm-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-perm-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.UpdateUserAdmin(jia.ID, true, false, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := db.UserByID(jia.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != jia.Key || got.Nickname != "甲" || !got.Banned || got.CanSendMessage || got.CanSendFile || got.CanCreateGroup {
		t.Fatalf("%+v", got)
	}
	if _, err := db.PostMessage(jia.ID, convID, "text", "封禁后", "", now); err == nil {
		t.Fatal("expected error for banned sender")
	}

	if err := db.UpdateUserAdmin(jia.ID, false, false, true, true); err != nil {
		t.Fatal(err)
	}
	got, err = db.UserByID(jia.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Banned || got.CanSendMessage || !got.CanSendFile || !got.CanCreateGroup {
		t.Fatalf("%+v", got)
	}
	if _, err := db.PostMessage(jia.ID, convID, "text", "禁言后", "", now); err == nil {
		t.Fatal("expected error when CanSendMessage is false")
	}

	if err := db.UpdateUserAdmin(jia.ID, false, true, false, false); err != nil {
		t.Fatal(err)
	}
	got, err = db.UserByID(jia.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Banned || !got.CanSendMessage || got.CanSendFile || got.CanCreateGroup {
		t.Fatalf("%+v", got)
	}
	if _, err := db.PostMessage(jia.ID, convID, "image", "", "file-image", now); err == nil {
		t.Fatal("expected error for image when CanSendFile is false")
	}
	if _, err := db.PostMessage(jia.ID, convID, "file", "notes.txt", "file-doc", now); err == nil {
		t.Fatal("expected error for file when CanSendFile is false")
	}
	msg, err := db.PostMessage(jia.ID, convID, "text", "文字仍可发", "", now)
	if err != nil || msg.Body != "文字仍可发" || msg.Kind != "text" {
		t.Fatalf("%+v %v", msg, err)
	}

	if err := db.UpdateUserAdmin(yi.ID, true, true, true, true); err != nil {
		t.Fatal(err)
	}
	peer, err := db.UserByID(yi.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !peer.Banned || !peer.CanSendMessage || !peer.CanSendFile || !peer.CanCreateGroup || peer.Nickname != "乙" {
		t.Fatalf("%+v", peer)
	}
	if _, err := db.PostMessage(jia.ID, convID, "text", "对方被封", "", now); err == nil {
		t.Fatal("expected error when the direct peer is banned")
	}

	if err := db.UpdateUserAdmin("missing-user", false, true, true, true); err == nil {
		t.Fatal("expected error when updating a missing user")
	}

	var messageCount int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("messages = %d, rejected sends were stored", messageCount)
	}
}

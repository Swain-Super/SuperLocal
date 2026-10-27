package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLoginOrCreateAndHideKey(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	first, err := db.LoginOrCreate("user-key1", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.LoginOrCreate("user-key1", "乙", now)
	if err != nil || second.Nickname != "甲" || second.ID != first.ID {
		t.Fatalf("%+v", second)
	}
	pub := Public(first)
	raw, _ := json.Marshal(pub)
	if strings.Contains(string(raw), "user-key1") || strings.Contains(string(raw), `"key"`) {
		t.Fatalf("public json leaked key: %s", raw)
	}
	me := MeView(first)
	if me.Key != "user-key1" {
		t.Fatal(me.Key)
	}
}

func TestLoginOrCreateRejectsNickname(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	cases := []string{"", "   ", "a\nb", "a\rb", strings.Repeat("字", 33)}
	for _, nick := range cases {
		if _, err := db.LoginOrCreate("user-key2", nick, now); err == nil {
			t.Fatalf("expected error for %q", nick)
		}
	}
	okNick := strings.Repeat("字", 32)
	u, err := db.LoginOrCreate("user-key2", "  "+okNick+"  ", now)
	if err != nil || u.Nickname != okNick {
		t.Fatalf("%+v %v", u, err)
	}
	if u.Banned || !u.CanSendMessage || !u.CanSendFile || !u.CanCreateGroup {
		t.Fatalf("%+v", u)
	}
	again, err := db.LoginOrCreate("user-key2", "", now)
	if err != nil || again.ID != u.ID || again.Nickname != okNick {
		t.Fatalf("%+v %v", again, err)
	}
	got, err := db.UserByID(u.ID)
	if err != nil || got.Key != "user-key2" || got.Nickname != okNick {
		t.Fatalf("%+v %v", got, err)
	}
	u.AvatarPath = "avatars/x.png"
	if Public(u).AvatarURL != "/api/avatars/"+u.ID || MeView(u).AvatarURL != "/api/avatars/"+u.ID {
		t.Fatalf("avatar %q %q", Public(u).AvatarURL, MeView(u).AvatarURL)
	}
	if Public(User{ID: u.ID}).AvatarURL != "" {
		t.Fatal(Public(User{ID: u.ID}).AvatarURL)
	}
}

package store

import (
	"testing"
	"time"
)

func TestFriendRequestAcceptAndProfile(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	jia, err := db.LoginOrCreate("friend-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("friend-yi", "乙", now)
	if err != nil {
		t.Fatal(err)
	}
	sig := "今天也要早点睡"
	pendant := "crown"
	updated, err := db.UpdateProfile(jia.ID, nil, &sig, &pendant, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Signature != sig || updated.Pendant != pendant || updated.Nickname != "甲" {
		t.Fatalf("profile %+v", updated)
	}
	if err := db.RequestFriend(jia.ID, yi.ID, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	incoming, err := db.ListFriends(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(incoming.Incoming) != 1 || incoming.Incoming[0].ID != jia.ID || len(incoming.Friends) != 0 {
		t.Fatalf("incoming %+v", incoming)
	}
	if err := db.RequestFriend(yi.ID, jia.ID, now.UnixMilli()); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListFriends(jia.ID, map[string]bool{yi.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Friends) != 1 || !list.Friends[0].Online || list.Friends[0].Nickname != "乙" || len(list.Incoming) != 0 {
		t.Fatalf("friends %+v", list)
	}
	if err := db.RemoveFriend(jia.ID, yi.ID); err != nil {
		t.Fatal(err)
	}
	after, err := db.ListFriends(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Friends) != 0 || len(after.Incoming) != 0 {
		t.Fatalf("after remove %+v", after)
	}
}

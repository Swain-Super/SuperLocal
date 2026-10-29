package store

import (
	"testing"
	"time"
)

func TestReplyRecallReactionAndPin(t *testing.T) {
	db := openTestDB(t)
	now := time.Unix(1_700_000_000, 0)
	jia, err := db.LoginOrCreate("user-key-feishu-jia", "甲", now)
	if err != nil {
		t.Fatal(err)
	}
	yi, err := db.LoginOrCreate("user-key-feishu-yi", "乙", now.Add(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	convID, err := db.OpenDirect(jia.ID, yi.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.PostMessage(jia.ID, convID, "text", "原话", "", now)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := db.PostReply(yi.ID, convID, "text", "引用", "", first.ID, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if reply.Reply == nil || reply.Reply.Body != "原话" || reply.Reply.SenderNickname != "甲" {
		t.Fatalf("reply = %+v", reply.Reply)
	}

	reacted, err := db.ToggleReaction(jia.ID, reply.ID, "ok", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(reacted.Reactions) != 1 || reacted.Reactions[0].Emoji != "ok" || reacted.Reactions[0].Count != 1 || !reacted.Reactions[0].Self {
		t.Fatalf("reactions = %+v", reacted.Reactions)
	}
	cleared, err := db.ToggleReaction(jia.ID, reply.ID, "ok", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared.Reactions) != 0 {
		t.Fatalf("reaction stayed: %+v", cleared.Reactions)
	}

	recalled, err := db.RecallMessage(jia.ID, first.ID, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !recalled.Recalled || recalled.Body != "" {
		t.Fatalf("recalled = %+v", recalled)
	}
	list, err := db.Messages(yi.ID, convID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || !list[0].Recalled || list[0].Body != "" || list[1].Reply == nil || !list[1].Reply.Recalled {
		t.Fatalf("messages = %+v reply %+v", list, list[1].Reply)
	}

	pinned := true
	if err := db.SetConversationFlags(yi.ID, convID, &pinned, nil); err != nil {
		t.Fatal(err)
	}
	views, err := db.ListConversations(yi.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || !views[0].Pinned || views[0].LastBody != "引用" {
		t.Fatalf("views = %+v", views)
	}
}

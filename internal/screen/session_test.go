package screen

import (
	"testing"
	"time"
)

func turnOf(string) Turn {
	return Turn{URLs: []string{"stun:127.0.0.1:8080"}, Username: "u", Credential: "p"}
}

func seqIDs(ids ...string) func(time.Time) string {
	i := 0
	return func(time.Time) string {
		id := ids[i]
		if i < len(ids)-1 {
			i++
		}
		return id
	}
}

func testSession(now time.Time, ids ...string) *Session {
	if len(ids) == 0 {
		ids = []string{"screen-1"}
	}
	return New(func() time.Time { return now }, seqIDs(ids...), turnOf)
}

func TestInviteRejectsGroupSizedMemberList(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	out := s.Invite("a", "甲", "conv", []string{"a", "b", "c"}, true)
	if len(out) != 1 || out[0].Event.Reason != "forbidden" || out[0].Event.Type != "screen.reject" {
		t.Fatalf("%+v", out)
	}
	outsider := s.Invite("c", "丙", "conv", []string{"a", "b"}, true)
	if outsider[0].Event.Reason != "forbidden" {
		t.Fatalf("%+v", outsider)
	}
}

func TestInviteOfflineBusyAndTimeout(t *testing.T) {
	now := time.Unix(100, 0)
	s := testSession(now, "screen-1", "screen-2")
	off := s.Invite("a", "甲", "conv", []string{"a", "b"}, false)
	if off[0].Event.Reason != "offline" {
		t.Fatalf("%+v", off)
	}
	ring := s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	if ring[0].Event.Type != "screen.ringing" || ring[1].Event.Type != "screen.invite" || ring[1].Event.ScreenID != "screen-1" {
		t.Fatalf("%+v", ring)
	}
	if ring[1].Event.FromNickname != "甲" || ring[0].Event.Turn == nil {
		t.Fatalf("%+v", ring)
	}
	busy := s.Invite("c", "丙", "other", []string{"c", "a"}, true)
	if busy[0].Event.Reason != "busy" {
		t.Fatalf("%+v", busy)
	}
	timed := s.Sweep(now.Add(31 * time.Second))
	if len(timed) != 1 || timed[0].Event.Reason != "timeout" || timed[0].Event.Type != "screen.reject" {
		t.Fatalf("%+v", timed)
	}
	again := s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	if again[0].Event.Type != "screen.ringing" || again[0].Event.ScreenID != "screen-2" {
		t.Fatalf("not free: %+v", again)
	}
}

func TestAcceptedShareDoesNotTimeout(t *testing.T) {
	now := time.Unix(100, 0)
	s := testSession(now)
	s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	s.Accept("b", "screen-1")
	if timed := s.Sweep(now.Add(time.Minute)); len(timed) != 0 {
		t.Fatalf("accepted timed out: %+v", timed)
	}
}

func TestRejectHangupAndDisconnect(t *testing.T) {
	s := testSession(time.Unix(100, 0), "screen-1", "screen-2", "screen-3")
	s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	rejected := s.Reject("b", "screen-1")
	if rejected[0].Event.Reason != "reject" || rejected[0].UserIDs[0] != "a" {
		t.Fatalf("%+v", rejected)
	}
	s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	hung := s.Hangup("a", "screen-2")
	if hung[0].Event.Type != "screen.hangup" || hung[0].UserIDs[0] != "b" {
		t.Fatalf("%+v", hung)
	}
	s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	gone := s.Disconnect("b")
	if gone[0].Event.Type != "screen.hangup" || gone[0].UserIDs[0] != "a" || gone[0].Event.ScreenID != "screen-3" {
		t.Fatalf("%+v", gone)
	}
}

func TestOfferWaitsForAcceptAndRoles(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	s.Invite("a", "甲", "conv", []string{"a", "b"}, true)
	if len(s.Offer("a", "screen-1", "offer")) != 0 {
		t.Fatal("offer before accept")
	}
	s.Accept("b", "screen-1")
	if len(s.Offer("b", "screen-1", "viewer")) != 0 {
		t.Fatal("viewer offer")
	}
	offered := s.Offer("a", "screen-1", "offer")
	if offered[0].Event.Type != "screen.offer" || offered[0].UserIDs[0] != "b" || offered[0].Event.SDP != "offer" {
		t.Fatalf("%+v", offered)
	}
	if len(s.Answer("a", "screen-1", "nope")) != 0 {
		t.Fatal("sharer answer")
	}
	answered := s.Answer("b", "screen-1", "answer")
	if answered[0].UserIDs[0] != "a" || answered[0].Event.SDP != "answer" {
		t.Fatalf("%+v", answered)
	}
}

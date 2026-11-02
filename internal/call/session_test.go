package call

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
		ids = []string{"call-1"}
	}
	return New(func() time.Time { return now }, seqIDs(ids...), turnOf)
}

func TestInviteRejectsGroupSizedMemberList(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	out := s.Invite("a", "甲", "conv", "audio", []string{"a", "b", "c"}, true)
	if len(out) != 1 || out[0].Event.Reason != "forbidden" || out[0].Event.Type != "call.reject" {
		t.Fatalf("%+v", out)
	}
}

func TestInviteOfflineBusyAndTimeout(t *testing.T) {
	now := time.Unix(100, 0)
	s := testSession(now, "call-1", "call-2")
	off := s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, false)
	if off[0].Event.Reason != "offline" {
		t.Fatalf("%+v", off)
	}
	ring := s.Invite("a", "甲", "conv", "video", []string{"a", "b"}, true)
	if ring[0].Event.Type != "call.ringing" || ring[1].Event.Type != "call.invite" || ring[1].Event.CallID != "call-1" {
		t.Fatalf("%+v", ring)
	}
	if ring[1].Event.FromNickname != "甲" || ring[0].Event.Turn == nil {
		t.Fatalf("%+v", ring)
	}
	busy := s.Invite("c", "丙", "other", "audio", []string{"c", "a"}, true)
	if busy[0].Event.Reason != "busy" {
		t.Fatalf("%+v", busy)
	}
	timed := s.Sweep(now.Add(31 * time.Second))
	if len(timed) != 1 || timed[0].Event.Reason != "timeout" {
		t.Fatalf("%+v", timed)
	}
	again := s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	if again[0].Event.Type != "call.ringing" || again[0].Event.CallID != "call-2" {
		t.Fatalf("not free: %+v", again)
	}
}

func TestRejectHangupAndDisconnect(t *testing.T) {
	s := testSession(time.Unix(100, 0), "call-1", "call-2", "call-3")
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	rejected := s.Reject("b", "call-1")
	if rejected[0].Event.Reason != "reject" || rejected[0].UserIDs[0] != "a" {
		t.Fatalf("%+v", rejected)
	}
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	hung := s.Hangup("a", "call-2")
	if hung[0].Event.Type != "call.hangup" || hung[0].UserIDs[0] != "b" {
		t.Fatalf("%+v", hung)
	}
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	gone := s.Disconnect("b")
	if gone[0].Event.Type != "call.hangup" || gone[0].UserIDs[0] != "a" || gone[0].Event.CallID != "call-3" {
		t.Fatalf("%+v", gone)
	}
}

func TestOfferWaitsForAccept(t *testing.T) {
	s := testSession(time.Unix(100, 0))
	s.Invite("a", "甲", "conv", "audio", []string{"a", "b"}, true)
	if len(s.Offer("a", "call-1", "offer")) != 0 {
		t.Fatal("offer before accept")
	}
	s.Accept("b", "call-1")
	offered := s.Offer("a", "call-1", "offer")
	if offered[0].Event.Type != "call.offer" || offered[0].UserIDs[0] != "b" || offered[0].Event.SDP != "offer" {
		t.Fatalf("%+v", offered)
	}
	answered := s.Answer("b", "call-1", "answer")
	if answered[0].UserIDs[0] != "a" || answered[0].Event.SDP != "answer" {
		t.Fatalf("%+v", answered)
	}
}

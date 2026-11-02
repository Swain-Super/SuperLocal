package turncred

import (
	"testing"
	"time"
)

func TestMintExpiresAndMatches(t *testing.T) {
	secret := []byte("secret")
	now := time.Unix(1_700_000_000, 0)
	username, credential := Mint(secret, "user-1", now)
	if !Verify(secret, username, credential, now.Add(4*time.Minute)) {
		t.Fatal("fresh credential rejected")
	}
	if Verify(secret, username, credential, now.Add(6*time.Minute)) {
		t.Fatal("expired credential accepted")
	}
	if Verify(secret, username, "nope", now) {
		t.Fatal("wrong credential accepted")
	}
}

func TestURLsUseLoopbackWhenNoLAN(t *testing.T) {
	got := URLs("8080", nil)
	if len(got) != 2 || got[0] != "stun:127.0.0.1:8080" || got[1] != "turn:127.0.0.1:8080?transport=udp" {
		t.Fatalf("%v", got)
	}
}

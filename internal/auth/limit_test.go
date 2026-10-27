package auth

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	lim := NewLimiter()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	for i := 1; i <= 20; i++ {
		if !lim.Allow("1.2.3.4", now) {
			t.Fatalf("attempt %d: want true", i)
		}
	}
	if lim.Allow("1.2.3.4", now) {
		t.Fatal("21st attempt: want false")
	}
	if !lim.Allow("5.6.7.8", now) {
		t.Fatal("other ip: want true")
	}
	if !lim.Allow("1.2.3.4", now.Add(61*time.Second)) {
		t.Fatal("after 61s: want true")
	}
}

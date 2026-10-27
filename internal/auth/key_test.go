package auth

import "testing"

func TestNormalizeKey(t *testing.T) {
	got, err := NormalizeKey("  ab_cd-12 ")
	if err != nil || got != "ab_cd-12" {
		t.Fatal(got, err)
	}
	if _, err := NormalizeKey("short"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NormalizeKey("has space!!"); err == nil {
		t.Fatal("expected error")
	}
}

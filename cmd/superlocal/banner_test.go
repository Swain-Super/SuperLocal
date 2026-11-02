package main

import (
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	got := banner(":8080", "admin", "secret", []string{"192.168.1.8"}, true)
	for _, want := range []string{
		"https://127.0.0.1:8080/",
		"https://127.0.0.1:8080/admin",
		"https://192.168.1.8:8080/",
		"https://192.168.1.8:8080/admin",
		"admin",
		"secret",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

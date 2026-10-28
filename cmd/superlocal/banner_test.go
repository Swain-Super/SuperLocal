package main

import (
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	got := banner(":8080", "admin", "secret", []string{"192.168.1.8"})
	for _, want := range []string{
		"http://127.0.0.1:8080/",
		"http://127.0.0.1:8080/admin",
		"http://192.168.1.8:8080/",
		"http://192.168.1.8:8080/admin",
		"admin",
		"secret",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q\n%s", want, got)
		}
	}
}

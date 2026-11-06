package main

import (
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	got := banner(":8080", "admin", "secret", []string{"192.168.1.8"}, true)
	want := strings.Join([]string{
		"https://127.0.0.1:8080/",
		"https://127.0.0.1:8080/admin",
		"http://127.0.0.1:8080/",
		"http://127.0.0.1:8080/admin",
		"https://192.168.1.8:8080/",
		"https://192.168.1.8:8080/admin",
		"http://192.168.1.8:8080/",
		"http://192.168.1.8:8080/admin",
		"账号 admin",
		"密码 secret",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s", got)
	}
}

func TestBannerHTTPOnly(t *testing.T) {
	got := banner(":8080", "admin", "pw", nil, false)
	if strings.Contains(got, "https://") {
		t.Fatalf("http-only banner included https:\n%s", got)
	}
	if !strings.HasPrefix(got, "http://127.0.0.1:8080/\nhttp://127.0.0.1:8080/admin\n") {
		t.Fatalf("got:\n%s", got)
	}
}

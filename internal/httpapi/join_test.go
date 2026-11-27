package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/httpapi"
)

func TestJoinURL(t *testing.T) {
	cases := []struct {
		host string
		ips  []string
		want string
	}{
		{"127.0.0.1:18080", []string{"192.168.1.8", "169.254.1.1", "::1"}, "http://192.168.1.8:18080/"},
		{"10.0.0.8:8080", []string{"192.168.1.8", "10.0.0.8"}, "http://10.0.0.8:8080/"},
		{"192.168.1.8", []string{"192.168.1.8"}, ""},
		{"127.0.0.1:8080", nil, ""},
		{"localhost:8080", []string{"169.254.1.1"}, ""},
	}
	for _, tc := range cases {
		if got := httpapi.JoinURL(tc.host, tc.ips); got != tc.want {
			t.Fatalf("JoinURL(%q, %v)=%q, want %q", tc.host, tc.ips, got, tc.want)
		}
	}
}

func TestJoinEndpoint(t *testing.T) {
	db := openHTTPDB(t)
	srv := httptest.NewServer(httpapi.New(httpapi.Deps{
		DB:        db,
		DataDir:   t.TempDir(),
		MaxUpload: 1024,
		Limiter:   auth.NewLimiter(),
		Now:       func() time.Time { return time.Unix(1_700_000_000, 0) },
	}))
	defer srv.Close()
	res, err := http.Get(srv.URL + "/api/join")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
}

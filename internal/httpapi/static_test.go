package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"superlocal/internal/httpapi"
)

func TestStaticIndexFallback(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := httpapi.WithStatic(next)

	for _, path := range []string{"/", "/admin", "/missing"} {
		res := requestStatic(t, h, path)
		if res.code != http.StatusOK || !strings.Contains(res.body, "SuperLocal") {
			t.Fatalf("%s status=%d body=%q", path, res.code, res.body)
		}
		if !strings.Contains(res.ctype, "text/html") {
			t.Fatalf("%s content-type %q", path, res.ctype)
		}
	}

	res := requestStatic(t, h, "/index.html")
	if res.code == http.StatusMovedPermanently {
		if res.loc == "" {
			t.Fatal("index.html redirect missing Location")
		}
	} else if res.code != http.StatusOK || !strings.Contains(res.body, "SuperLocal") {
		t.Fatalf("index status=%d body=%q", res.code, res.body)
	}

	res = requestStatic(t, h, "/api/session")
	if res.code != http.StatusNoContent {
		t.Fatalf("api status=%d body=%q", res.code, res.body)
	}
}

type staticResponse struct {
	code  int
	body  string
	ctype string
	loc   string
}

func requestStatic(t *testing.T, h http.Handler, path string) staticResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	return staticResponse{
		code:  rr.Code,
		body:  string(body),
		ctype: rr.Header().Get("Content-Type"),
		loc:   rr.Header().Get("Location"),
	}
}

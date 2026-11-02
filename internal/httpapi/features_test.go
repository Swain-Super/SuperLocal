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

func TestFeaturesReflectCalls(t *testing.T) {
	db := openHTTPDB(t)
	for _, enabled := range []bool{true, false} {
		srv := httptest.NewServer(httpapi.New(httpapi.Deps{
			DB:           db,
			DataDir:      t.TempDir(),
			MaxUpload:    1024,
			Limiter:      auth.NewLimiter(),
			Now:          func() time.Time { return time.Unix(1_700_000_000, 0) },
			CallsEnabled: enabled,
		}))
		res, err := http.Get(srv.URL + "/api/features")
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Calls bool `json:"calls"`
		}
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		srv.Close()
		if body.Calls != enabled {
			t.Fatalf("calls %v, want %v", body.Calls, enabled)
		}
	}
}

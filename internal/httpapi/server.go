package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"superlocal/internal/auth"
	"superlocal/internal/hub"
	"superlocal/internal/store"
)

type Deps struct {
	DB        *store.DB
	Hub       *hub.Hub
	DataDir   string
	MaxUpload int64
	Limiter   *auth.Limiter
	Now       func() time.Time
}

type server struct {
	db        *store.DB
	hub       *hub.Hub
	dataDir   string
	maxUpload int64
	limiter   *auth.Limiter
	now       func() time.Time
}

func New(deps Deps) http.Handler {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	s := &server{
		db:        deps.DB,
		hub:       deps.Hub,
		dataDir:   deps.DataDir,
		maxUpload: deps.MaxUpload,
		limiter:   deps.Limiter,
		now:       now,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/session", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)

	mux.HandleFunc("POST /api/admin/session", s.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", s.handleAdminLogout)
	mux.HandleFunc("POST /api/admin/password", s.handleAdminPassword)
	mux.HandleFunc("GET /api/admin/stats", s.handleAdminStats)
	mux.HandleFunc("GET /api/admin/users", s.handleAdminUsers)
	mux.HandleFunc("PATCH /api/admin/users/{id}", s.handleAdminPatchUser)
	mux.HandleFunc("GET /api/admin/groups", s.handleAdminGroups)
	mux.HandleFunc("DELETE /api/admin/groups/{id}", s.handleAdminDeleteGroup)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *server) clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	if i := len(host) - 1; i >= 0 {
		for j := i; j >= 0; j-- {
			if host[j] == ':' {
				return host[:j]
			}
		}
	}
	return host
}

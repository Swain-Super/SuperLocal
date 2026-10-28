package httpapi

import (
	"encoding/json"
	"net"
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
	mux.HandleFunc("PATCH /api/me", s.handlePatchMe)
	mux.HandleFunc("POST /api/me/avatar", s.handleUploadAvatar)

	mux.HandleFunc("GET /api/users/online", s.handleOnlineUsers)
	mux.HandleFunc("GET /api/users", s.handleSearchUsers)

	mux.HandleFunc("POST /api/conversations/direct", s.handleOpenDirect)
	mux.HandleFunc("GET /api/conversations", s.handleListConversations)
	mux.HandleFunc("GET /api/conversations/{id}/messages", s.handleMessages)
	mux.HandleFunc("POST /api/conversations/{id}/read", s.handleMarkRead)

	mux.HandleFunc("POST /api/groups", s.handleCreateGroup)
	mux.HandleFunc("POST /api/groups/{id}/members", s.handleAddMembers)
	mux.HandleFunc("POST /api/groups/{id}/leave", s.handleLeaveGroup)

	mux.HandleFunc("POST /api/files", s.handleUpload)
	mux.HandleFunc("GET /api/files/{id}", s.handleDownload)
	mux.HandleFunc("GET /api/avatars/{id}", s.handleGetAvatar)

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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

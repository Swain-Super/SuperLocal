package httpapi

import (
	"encoding/json"
	"net/http"

	"superlocal/internal/auth"
	"superlocal/internal/store"
)

type loginRequest struct {
	Key      string `json:"key"`
	Nickname string `json:"nickname"`
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	now := s.now()
	if s.limiter != nil && !s.limiter.Allow(ip, now) {
		writeError(w, http.StatusTooManyRequests, "请稍后再试")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	key, err := auth.NormalizeKey(req.Key)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.db.LoginOrCreate(key, req.Nickname, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if user.Banned {
		writeError(w, http.StatusForbidden, "账号已被封禁")
		return
	}
	token, err := s.db.CreateSession("user", user.ID, now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "登录失败")
		return
	}
	setSessionCookie(w, cookieSession, token)
	writeJSON(w, http.StatusOK, struct {
		store.Me
		Token string `json:"token"`
	}{Me: store.MeView(user), Token: token})
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := userToken(r)
	if token != "" {
		_ = s.db.DeleteSession(token)
	}
	clearCookie(w, cookieSession)
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.meView(user, r))
}

func (s *server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Nickname  *string `json:"nickname"`
		Signature *string `json:"signature"`
		Pendant   *string `json:"pendant"`
		Dnd       *bool   `json:"dnd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if req.Nickname == nil && req.Signature == nil && req.Pendant == nil && req.Dnd == nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	updated, err := s.db.UpdateProfile(user.ID, req.Nickname, req.Signature, req.Pendant, req.Dnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.broadcastDnd(updated.ID, updated.Dnd)
	writeJSON(w, http.StatusOK, s.meView(updated, r))
}

func (s *server) meView(user store.User, r *http.Request) store.Me {
	me := store.MeView(user)
	me.IP = s.clientIP(r)
	return me
}

func (s *server) requireUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	token := userToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "未登录")
		return store.User{}, false
	}
	kind, userID, err := s.db.LookupSession(token, s.now())
	if err != nil || kind != "user" || userID == "" {
		writeError(w, http.StatusUnauthorized, "未登录")
		return store.User{}, false
	}
	user, err := s.db.UserByID(userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "未登录")
		return store.User{}, false
	}
	if user.Banned {
		writeError(w, http.StatusForbidden, "账号已被封禁")
		return store.User{}, false
	}
	return user, true
}

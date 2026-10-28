package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"superlocal/internal/auth"
)

type adminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type adminPasswordRequest struct {
	Password string `json:"password"`
}

type adminPatchUserRequest struct {
	Banned         bool `json:"banned"`
	CanSendMessage bool `json:"canSendMessage"`
	CanSendFile    bool `json:"canSendFile"`
	CanCreateGroup bool `json:"canCreateGroup"`
}

type adminUserView struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Nickname       string    `json:"nickname"`
	Online         bool      `json:"online"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
	Banned         bool      `json:"banned"`
	CanSendMessage bool      `json:"canSendMessage"`
	CanSendFile    bool      `json:"canSendFile"`
	CanCreateGroup bool      `json:"canCreateGroup"`
}

type adminGroupView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	MemberCount int       `json:"memberCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (s *server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := auth.CheckAdmin(s.db, req.Username, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "登录失败")
		return
	}
	token, err := s.db.CreateSession("admin", "", s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "登录失败")
		return
	}
	setSessionCookie(w, cookieAdmin, token)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	token := readCookie(r, cookieAdmin)
	if token != "" {
		_ = s.db.DeleteSession(token)
	}
	clearCookie(w, cookieAdmin)
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleAdminPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req adminPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := auth.ChangeAdminPassword(s.db, s.dataDir, req.Password); err != nil {
		writeError(w, http.StatusInternalServerError, "修改失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	now := s.now()
	cutoff := now.Add(-48 * time.Hour).UnixMilli()

	var users, banned, messages int
	var fileBytes int64
	if err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM users WHERE banned = 1`).Scan(&banned); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM messages WHERE created_at >= ?`, cutoff,
	).Scan(&messages); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	if err := s.db.SQL().QueryRow(
		`SELECT COALESCE(SUM(size), 0) FROM files WHERE created_at >= ?`, cutoff,
	).Scan(&fileBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}

	online := 0
	if s.hub != nil {
		online = len(s.hub.OnlineIDs())
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users":     users,
		"online":    online,
		"banned":    banned,
		"messages":  messages,
		"fileBytes": fileBytes,
	})
}

func (s *server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var (
		rows *sql.Rows
		err  error
	)
	if q == "" {
		rows, err = s.db.SQL().Query(
			`SELECT id, key, nickname, banned, can_send_message, can_send_file, can_create_group, last_seen_at
			 FROM users ORDER BY created_at DESC`,
		)
	} else {
		like := "%" + q + "%"
		rows, err = s.db.SQL().Query(
			`SELECT id, key, nickname, banned, can_send_message, can_send_file, can_create_group, last_seen_at
			 FROM users
			 WHERE key LIKE ? OR nickname LIKE ?
			 ORDER BY created_at DESC`,
			like, like,
		)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()

	onlineIDs := map[string]bool{}
	if s.hub != nil {
		onlineIDs = s.hub.OnlineIDs()
	}

	users := []adminUserView{}
	for rows.Next() {
		var u adminUserView
		var banned, canSend, canFile, canGroup int
		var lastSeenMS int64
		if err := rows.Scan(
			&u.ID, &u.Key, &u.Nickname,
			&banned, &canSend, &canFile, &canGroup, &lastSeenMS,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "查询失败")
			return
		}
		u.Banned = banned != 0
		u.CanSendMessage = canSend != 0
		u.CanSendFile = canFile != 0
		u.CanCreateGroup = canGroup != 0
		if lastSeenMS != 0 {
			u.LastSeenAt = time.UnixMilli(lastSeenMS)
		}
		u.Online = onlineIDs[u.ID]
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (s *server) handleAdminPatchUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	var req adminPatchUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.UpdateUserAdmin(id, req.Banned, req.CanSendMessage, req.CanSendFile, req.CanCreateGroup); err != nil {
		if strings.Contains(err.Error(), "用户不存在") {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "更新失败")
		return
	}
	if req.Banned && s.hub != nil {
		s.hub.Kick(id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleAdminGroups(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	rows, err := s.db.SQL().Query(
		`SELECT c.id, c.name, c.created_at,
		        (SELECT COUNT(*) FROM members m WHERE m.conversation_id = c.id) AS member_count
		 FROM conversations c
		 WHERE c.type = 'group' AND c.dissolved = 0
		 ORDER BY c.created_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	defer rows.Close()

	groups := []adminGroupView{}
	for rows.Next() {
		var g adminGroupView
		var createdMS int64
		if err := rows.Scan(&g.ID, &g.Name, &createdMS, &g.MemberCount); err != nil {
			writeError(w, http.StatusInternalServerError, "查询失败")
			return
		}
		if createdMS != 0 {
			g.CreatedAt = time.UnixMilli(createdMS)
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (s *server) handleAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.Dissolve(id); err != nil {
		if strings.Contains(err.Error(), "会话不存在") {
			writeError(w, http.StatusNotFound, "会话不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "解散失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	token := readCookie(r, cookieAdmin)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "未登录")
		return false
	}
	kind, _, err := s.db.LookupSession(token, s.now())
	if err != nil || kind != "admin" {
		writeError(w, http.StatusUnauthorized, "未登录")
		return false
	}
	return true
}

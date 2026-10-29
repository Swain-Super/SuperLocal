package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"superlocal/internal/store"
)

type directRequest struct {
	UserID string `json:"userId"`
}

type groupCreateRequest struct {
	Name      string   `json:"name"`
	MemberIDs []string `json:"memberIds"`
}

type groupMembersRequest struct {
	MemberIDs []string `json:"memberIds"`
}

type readRequest struct {
	MessageID string `json:"messageId"`
}

func (s *server) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	_ = user
	online := s.hub.OnlineIDs()
	out := make([]store.PublicUser, 0, len(online))
	for id := range online {
		u, err := s.db.UserByID(id)
		if err != nil || u.Banned {
			continue
		}
		pub := store.Public(u)
		pub.Online = true
		out = append(out, pub)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireUser(w, r); !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	users, err := s.db.SearchUsers(q)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "搜索失败")
		return
	}
	if users == nil {
		users = []store.PublicUser{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *server) handleOpenDirect(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req directRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	id, err := s.db.OpenDirect(user.ID, req.UserID, s.now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	list, err := s.db.ListConversations(user.ID, s.hub.OnlineIDs())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载失败")
		return
	}
	if list == nil {
		list = []store.ConversationView{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req groupCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	id, err := s.db.CreateGroup(user.ID, req.Name, req.MemberIDs, s.now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id})
}

func (s *server) handleAddMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	convID := r.PathValue("id")
	var req groupMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.AddMembers(user.ID, convID, req.MemberIDs, s.now()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if err := s.db.RemoveMember(user.ID, r.PathValue("id"), r.PathValue("userId")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleLeaveGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	convID := r.PathValue("id")
	if err := s.db.Leave(user.ID, convID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	list, err := s.db.Members(user.ID, r.PathValue("id"), s.hub.OnlineIDs())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if list == nil {
		list = []store.PublicUser{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleConversationFlags(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Pinned *bool `json:"pinned"`
		Muted  *bool `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.SetConversationFlags(user.ID, r.PathValue("id"), req.Pinned, req.Muted); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

func (s *server) handleMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	convID := r.PathValue("id")
	after := r.URL.Query().Get("after")
	msgs, err := s.db.Messages(user.ID, convID, after)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if msgs == nil {
		msgs = []store.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

func (s *server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	convID := r.PathValue("id")
	var req readRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.MarkRead(user.ID, convID, req.MessageID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "1"})
}

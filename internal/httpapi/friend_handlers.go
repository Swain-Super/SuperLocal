package httpapi

import (
	"encoding/json"
	"net/http"
)

func (s *server) handleFriends(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	list, err := s.db.ListFriends(user.ID, s.hub.OnlineIDs())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取好友失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleRequestFriend(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.RequestFriend(user.ID, req.UserID, s.now().UnixMilli()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := s.db.ListFriends(user.ID, s.hub.OnlineIDs())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取好友失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *server) handleRemoveFriend(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	otherID := r.PathValue("id")
	if otherID == "" {
		writeError(w, http.StatusBadRequest, "请求无效")
		return
	}
	if err := s.db.RemoveFriend(user.ID, otherID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := s.db.ListFriends(user.ID, s.hub.OnlineIDs())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取好友失败")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

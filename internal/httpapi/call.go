package httpapi

import (
	"encoding/json"

	"github.com/coder/websocket"

	"superlocal/internal/call"
	"superlocal/internal/store"
)

func (s *server) handleCall(conn *websocket.Conn, user store.User, in wsIncoming) {
	if s.calls == nil || !s.isSecure(conn) {
		s.writeWSError(conn, "通话不可用")
		return
	}
	var deliveries []call.Delivery
	switch in.Type {
	case "call.invite":
		members, peerOnline := s.callMembers(in.ConversationID, user.ID)
		deliveries = s.calls.Invite(user.ID, user.Nickname, in.ConversationID, in.Kind, members, peerOnline)
	case "call.accept":
		deliveries = s.calls.Accept(user.ID, in.CallID)
	case "call.reject":
		deliveries = s.calls.Reject(user.ID, in.CallID)
	case "call.hangup":
		deliveries = s.calls.Hangup(user.ID, in.CallID)
	case "call.offer":
		deliveries = s.calls.Offer(user.ID, in.CallID, in.SDP)
	case "call.answer":
		deliveries = s.calls.Answer(user.ID, in.CallID, in.SDP)
	case "call.candidate":
		deliveries = s.calls.Candidate(user.ID, in.CallID, in.Candidate)
	default:
		return
	}
	s.dispatchCall(deliveries)
}

func (s *server) callMembers(convID, selfID string) ([]string, bool) {
	var convType string
	if err := s.db.SQL().QueryRow(`SELECT type FROM conversations WHERE id = ?`, convID).Scan(&convType); err != nil || convType != "direct" {
		return nil, false
	}
	members, err := s.conversationMemberIDs(convID)
	if err != nil || len(members) != 2 {
		return nil, false
	}
	peer := members[0]
	if peer == selfID {
		peer = members[1]
	}
	return members, s.secureOnline(peer)
}

func (s *server) dispatchCall(deliveries []call.Delivery) {
	for _, delivery := range deliveries {
		payload, err := json.Marshal(delivery.Event)
		if err != nil {
			continue
		}
		s.sendSecure(delivery.UserIDs, payload)
	}
}

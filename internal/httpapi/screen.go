package httpapi

import (
	"encoding/json"

	"github.com/coder/websocket"

	"superlocal/internal/screen"
	"superlocal/internal/store"
)

func (s *server) handleScreen(conn *websocket.Conn, user store.User, in wsIncoming) {
	if s.screens == nil {
		s.writeWSError(conn, "屏幕共享不可用")
		return
	}
	var deliveries []screen.Delivery
	switch in.Type {
	case "screen.invite":
		members, peerOnline := s.callMembers(in.ConversationID, user.ID)
		deliveries = s.screens.Invite(user.ID, user.Nickname, in.ConversationID, members, peerOnline)
	case "screen.accept":
		deliveries = s.screens.Accept(user.ID, in.ScreenID)
	case "screen.reject":
		deliveries = s.screens.Reject(user.ID, in.ScreenID)
	case "screen.hangup":
		deliveries = s.screens.Hangup(user.ID, in.ScreenID)
	case "screen.offer":
		deliveries = s.screens.Offer(user.ID, in.ScreenID, in.SDP)
	case "screen.answer":
		deliveries = s.screens.Answer(user.ID, in.ScreenID, in.SDP)
	case "screen.candidate":
		deliveries = s.screens.Candidate(user.ID, in.ScreenID, in.Candidate)
	default:
		return
	}
	s.dispatchScreen(deliveries)
}

func (s *server) dispatchScreen(deliveries []screen.Delivery) {
	if s.hub == nil {
		return
	}
	for _, delivery := range deliveries {
		payload, err := json.Marshal(delivery.Event)
		if err != nil {
			continue
		}
		s.hub.Send(delivery.UserIDs, payload)
	}
}

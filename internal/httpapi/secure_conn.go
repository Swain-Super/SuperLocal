package httpapi

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

func (s *server) trackSecure(userID string, conn *websocket.Conn) {
	if userID == "" || conn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.secureUsers[userID]
	if set == nil {
		set = map[*websocket.Conn]struct{}{}
		s.secureUsers[userID] = set
	}
	set[conn] = struct{}{}
	s.secureConns[conn] = struct{}{}
}

func (s *server) dropConn(userID string, conn *websocket.Conn) bool {
	s.mu.Lock()
	delete(s.secureConns, conn)
	if set := s.secureUsers[userID]; set != nil {
		delete(set, conn)
		if len(set) == 0 {
			delete(s.secureUsers, userID)
		}
	}
	s.mu.Unlock()
	if s.hub == nil {
		return true
	}
	return s.hub.Remove(userID, conn)
}

func (s *server) isSecure(conn *websocket.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secureConns[conn]
	return ok
}

func (s *server) secureOnline(userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.secureUsers[userID]) > 0
}

func (s *server) sendSecure(userIDs []string, payload []byte) {
	if len(payload) == 0 {
		return
	}
	s.mu.Lock()
	targets := make([]*websocket.Conn, 0)
	for _, id := range userIDs {
		for conn := range s.secureUsers[id] {
			targets = append(targets, conn)
		}
	}
	s.mu.Unlock()
	for _, conn := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = conn.Write(ctx, websocket.MessageText, payload)
		cancel()
	}
}

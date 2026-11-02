package call

import (
	"encoding/json"
	"sync"
	"time"
)

type Turn struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

type Event struct {
	Type           string          `json:"type"`
	CallID         string          `json:"callId,omitempty"`
	ConversationID string          `json:"conversationId,omitempty"`
	Kind           string          `json:"kind,omitempty"`
	FromUserID     string          `json:"fromUserId,omitempty"`
	FromNickname   string          `json:"fromNickname,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	SDP            string          `json:"sdp,omitempty"`
	Candidate      json.RawMessage `json:"candidate,omitempty"`
	Turn           *Turn           `json:"turn,omitempty"`
}

type Delivery struct {
	UserIDs []string
	Event   Event
}

type Session struct {
	mu     sync.Mutex
	now    func() time.Time
	id     func(time.Time) string
	turn   func(string) Turn
	calls  map[string]*liveCall
	byUser map[string]string
}

type liveCall struct {
	id             string
	conversationID string
	kind           string
	caller         string
	callee         string
	nickname       string
	accepted       bool
	started        time.Time
}

func New(now func() time.Time, id func(time.Time) string, turn func(string) Turn) *Session {
	if now == nil {
		now = time.Now
	}
	return &Session{
		now:    now,
		id:     id,
		turn:   turn,
		calls:  map[string]*liveCall{},
		byUser: map[string]string{},
	}
}

func (s *Session) Invite(callerID, nickname, conversationID, kind string, memberIDs []string, peerOnline bool) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validInvite(callerID, kind, memberIDs) {
		return []Delivery{rejectOnly(callerID, "", "forbidden")}
	}
	peer := otherMember(callerID, memberIDs)
	if !peerOnline {
		return []Delivery{rejectOnly(callerID, "", "offline")}
	}
	if _, ok := s.byUser[callerID]; ok {
		return []Delivery{rejectOnly(callerID, "", "busy")}
	}
	if _, ok := s.byUser[peer]; ok {
		return []Delivery{rejectOnly(callerID, "", "busy")}
	}
	now := s.now()
	item := &liveCall{
		id:             s.id(now),
		conversationID: conversationID,
		kind:           kind,
		caller:         callerID,
		callee:         peer,
		nickname:       nickname,
		started:        now,
	}
	s.calls[item.id] = item
	s.byUser[callerID] = item.id
	s.byUser[peer] = item.id
	creds := s.turn(callerID)
	return []Delivery{
		{UserIDs: []string{callerID}, Event: Event{Type: "call.ringing", CallID: item.id, ConversationID: conversationID, Turn: &creds}},
		{UserIDs: []string{peer}, Event: Event{Type: "call.invite", CallID: item.id, ConversationID: conversationID, Kind: kind, FromUserID: callerID, FromNickname: nickname, Turn: &creds}},
	}
}

func (s *Session) Accept(userID, callID string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || item.callee != userID || item.accepted {
		return nil
	}
	item.accepted = true
	return []Delivery{
		{UserIDs: []string{item.caller}, Event: Event{Type: "call.accept", CallID: item.id}},
		{UserIDs: []string{item.callee}, Event: Event{Type: "call.end", CallID: item.id}},
	}
}

func (s *Session) Reject(userID, callID string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || item.callee != userID || item.accepted {
		return nil
	}
	s.delete(item)
	return []Delivery{rejectOnly(item.caller, item.id, "reject")}
}

func (s *Session) Hangup(userID, callID string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || (item.caller != userID && item.callee != userID) {
		return nil
	}
	other := item.callee
	if userID == item.callee {
		other = item.caller
	}
	id := item.id
	s.delete(item)
	return []Delivery{{UserIDs: []string{other}, Event: Event{Type: "call.hangup", CallID: id}}}
}

func (s *Session) Offer(userID, callID, sdp string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || !item.accepted || item.caller != userID {
		return nil
	}
	return []Delivery{{UserIDs: []string{item.callee}, Event: Event{Type: "call.offer", CallID: item.id, SDP: sdp}}}
}

func (s *Session) Answer(userID, callID, sdp string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || !item.accepted || item.callee != userID {
		return nil
	}
	return []Delivery{{UserIDs: []string{item.caller}, Event: Event{Type: "call.answer", CallID: item.id, SDP: sdp}}}
}

func (s *Session) Candidate(userID, callID string, candidate json.RawMessage) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.calls[callID]
	if item == nil || !item.accepted || (item.caller != userID && item.callee != userID) {
		return nil
	}
	other := item.callee
	if userID == item.callee {
		other = item.caller
	}
	return []Delivery{{UserIDs: []string{other}, Event: Event{Type: "call.candidate", CallID: item.id, Candidate: candidate}}}
}

func (s *Session) Disconnect(userID string) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	callID := s.byUser[userID]
	item := s.calls[callID]
	if item == nil {
		return nil
	}
	other := item.callee
	if userID == item.callee {
		other = item.caller
	}
	id := item.id
	s.delete(item)
	return []Delivery{{UserIDs: []string{other}, Event: Event{Type: "call.hangup", CallID: id}}}
}

func (s *Session) Sweep(now time.Time) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Delivery
	for _, item := range s.calls {
		if item.accepted || now.Before(item.started.Add(30*time.Second)) {
			continue
		}
		out = append(out, Delivery{
			UserIDs: []string{item.caller, item.callee},
			Event:   Event{Type: "call.reject", CallID: item.id, Reason: "timeout"},
		})
		s.delete(item)
	}
	return out
}

func (s *Session) delete(item *liveCall) {
	delete(s.calls, item.id)
	delete(s.byUser, item.caller)
	delete(s.byUser, item.callee)
}

func validInvite(callerID, kind string, memberIDs []string) bool {
	if kind != "audio" && kind != "video" {
		return false
	}
	if len(memberIDs) != 2 {
		return false
	}
	return memberIDs[0] == callerID || memberIDs[1] == callerID
}

func otherMember(callerID string, memberIDs []string) string {
	if memberIDs[0] == callerID {
		return memberIDs[1]
	}
	return memberIDs[0]
}

func rejectOnly(userID, callID, reason string) Delivery {
	return Delivery{UserIDs: []string{userID}, Event: Event{Type: "call.reject", CallID: callID, Reason: reason}}
}

package auth

import (
	"sync"
	"time"
)

const maxLoginsPerMinute = 20

// Limiter 按 IP 限制每分钟登录次数。
type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{hits: make(map[string][]time.Time)}
}

// Allow 在 now 往前一分钟内，同一 IP 的记录少于 20 条时记入并返回 true。
func (l *Limiter) Allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-time.Minute)
	var kept []time.Time
	for _, at := range l.hits[ip] {
		if !at.Before(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= maxLoginsPerMinute {
		l.hits[ip] = kept
		return false
	}
	l.hits[ip] = append(kept, now)
	return true
}

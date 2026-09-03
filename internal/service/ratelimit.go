package service

import (
	"sync"
	"time"
)

// RateLimiter 提供按 key 的固定窗口限流与超限冷却。
type RateLimiter struct {
	mu       sync.Mutex
	states   map[string]*rateState
	limit    int
	window   time.Duration
	cooldown time.Duration
}

type rateState struct {
	windowStart time.Time
	count       int
	lockedUntil time.Time
}

func NewRateLimiter(limit int, window, cooldown time.Duration) *RateLimiter {
	return &RateLimiter{
		states:   make(map[string]*rateState),
		limit:    limit,
		window:   window,
		cooldown: cooldown,
	}
}

// Allow 判断 key 是否允许通过；在一个窗口内超过 limit 次后进入冷却期返回 false。
func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	st := r.states[key]
	if st == nil {
		st = &rateState{windowStart: now}
		r.states[key] = st
	}
	if now.Before(st.lockedUntil) {
		return false
	}
	if now.Sub(st.windowStart) >= r.window {
		st.windowStart = now
		st.count = 0
	}
	st.count++
	if st.count > r.limit {
		st.lockedUntil = now.Add(r.cooldown)
		st.windowStart = now
		st.count = 0
		return false
	}
	return true
}
package public

import (
	"net"
	"strings"
	"sync"
	"time"
)

const (
	loginWindow       = 5 * time.Minute
	loginIPLimit      = 100
	loginAccountLimit = 20
	maxLoginKeys      = 10000
)

type loginWindowState struct {
	started time.Time
	count   int
}

type loginAttemptLimiter struct {
	mu      sync.Mutex
	windows map[string]loginWindowState
}

var passwordLoginLimiter = &loginAttemptLimiter{windows: make(map[string]loginWindowState)}
var passwordHashSlots = make(chan struct{}, 4)

func remoteLoginIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func (l *loginAttemptLimiter) Allow(ip, username string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = make(map[string]loginWindowState)
	}
	keys := [2]string{"ip:" + ip, "account:" + strings.ToLower(username)}
	limits := [2]int{loginIPLimit, loginAccountLimit}
	for i, key := range keys {
		state, exists := l.windows[key]
		if exists && now.Sub(state.started) < loginWindow && state.count >= limits[i] {
			return false
		}
	}
	if len(l.windows) >= maxLoginKeys {
		for key, state := range l.windows {
			if now.Sub(state.started) >= loginWindow {
				delete(l.windows, key)
			}
		}
		if len(l.windows) >= maxLoginKeys {
			return false
		}
	}
	for _, key := range keys {
		state, exists := l.windows[key]
		if !exists || now.Sub(state.started) >= loginWindow {
			state = loginWindowState{started: now}
		}
		state.count++
		l.windows[key] = state
	}
	return true
}

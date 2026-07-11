package main

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type requestSecurityPolicy struct {
	allowedHosts   map[string]struct{}
	allowedOrigins map[string]struct{}
}

func newRequestSecurityPolicy(addr string) requestSecurityPolicy {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8090"
	}
	hosts := []string{"127.0.0.1:" + port, "localhost:" + port, "[::1]:" + port}
	p := requestSecurityPolicy{allowedHosts: map[string]struct{}{}, allowedOrigins: map[string]struct{}{}}
	for _, host := range hosts {
		p.allowedHosts[strings.ToLower(host)] = struct{}{}
		p.allowedOrigins["http://"+strings.ToLower(host)] = struct{}{}
	}
	return p
}

func normalizeRequestHost(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if strings.HasSuffix(raw, ".") {
		raw = strings.TrimSuffix(raw, ".")
	}
	return raw
}

func (p requestSecurityPolicy) validHost(raw string) bool {
	_, ok := p.allowedHosts[normalizeRequestHost(raw)]
	return ok
}

func (p requestSecurityPolicy) validOrigin(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	_, ok := p.allowedOrigins[strings.ToLower(u.Scheme+"://"+u.Host)]
	return ok
}

type operationLimiter struct {
	mu     sync.Mutex
	active map[string]bool
	last   map[string]time.Time
}

func newOperationLimiter() *operationLimiter {
	return &operationLimiter{active: map[string]bool{}, last: map[string]time.Time{}}
}

func expensiveOperation(path string) (string, time.Duration, bool) {
	switch path {
	case "/api/weather/refresh":
		return "weather-refresh", 15 * time.Second, true
	case "/api/cache/rebuild", "/api/calendars/sync", "/api/calendars/private/sync", "/api/calendars/private/discover", "/api/calendars/private/repair":
		return "calendar-operation", 20 * time.Second, true
	case "/api/backup", "/api/backup/restore", "/api/system-update", "/api/update", "/api/diagnostics", "/api/doctor":
		return "system-operation", 30 * time.Second, true
	default:
		return "", 0, false
	}
}

func (l *operationLimiter) begin(path string) (func(), time.Duration, bool) {
	key, cooldown, limited := expensiveOperation(path)
	if !limited {
		return func() {}, 0, true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[key] {
		return nil, cooldown, false
	}
	if prior := l.last[key]; !prior.IsZero() && now.Sub(prior) < cooldown {
		return nil, cooldown - now.Sub(prior), false
	}
	l.active[key] = true
	return func() { l.mu.Lock(); delete(l.active, key); l.last[key] = time.Now(); l.mu.Unlock() }, 0, true
}

func writeRateLimited(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, "operation temporarily rate limited", http.StatusTooManyRequests)
}

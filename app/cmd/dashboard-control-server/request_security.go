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

type operationClass string

const (
	operationWeather  operationClass = "weather-refresh"
	operationCalendar operationClass = "calendar-operation"
	operationSystem   operationClass = "system-operation"
)

type operationPolicy struct {
	Class    operationClass
	Cooldown time.Duration
}

var operationPolicies = map[string]operationPolicy{
	"/api/weather/refresh":            {Class: operationWeather, Cooldown: 15 * time.Second},
	"/api/cache/rebuild":              {Class: operationCalendar, Cooldown: 20 * time.Second},
	"/api/calendars/sync":             {Class: operationCalendar, Cooldown: 20 * time.Second},
	"/api/calendars/private/sync":     {Class: operationCalendar, Cooldown: 20 * time.Second},
	"/api/calendars/private/discover": {Class: operationCalendar, Cooldown: 20 * time.Second},
	"/api/calendars/private/repair":   {Class: operationCalendar, Cooldown: 20 * time.Second},
	"/api/backup":                     {Class: operationSystem, Cooldown: 30 * time.Second},
	"/api/backup/restore":             {Class: operationSystem, Cooldown: 30 * time.Second},
	"/api/system-update":              {Class: operationSystem, Cooldown: 30 * time.Second},
	"/api/update":                     {Class: operationSystem, Cooldown: 30 * time.Second},
	"/api/diagnostics":                {Class: operationSystem, Cooldown: 30 * time.Second},
	"/api/doctor":                     {Class: operationSystem, Cooldown: 30 * time.Second},
}

type operationLimiter struct {
	mu     sync.Mutex
	active map[operationClass]bool
	last   map[operationClass]time.Time
}

type operationLease struct {
	limiter  *operationLimiter
	policy   operationPolicy
	started  bool
	finished bool
}

func newOperationLimiter() *operationLimiter {
	return &operationLimiter{active: map[operationClass]bool{}, last: map[operationClass]time.Time{}}
}

func (l *operationLimiter) acquire(path string) (*operationLease, time.Duration, bool) {
	policy, limited := operationPolicies[path]
	if !limited {
		return &operationLease{}, 0, true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[policy.Class] {
		return nil, policy.Cooldown, false
	}
	if prior := l.last[policy.Class]; !prior.IsZero() && now.Sub(prior) < policy.Cooldown {
		return nil, policy.Cooldown - now.Sub(prior), false
	}
	l.active[policy.Class] = true
	return &operationLease{limiter: l, policy: policy}, 0, true
}

func (lease *operationLease) Start() {
	if lease != nil {
		lease.started = true
	}
}

func (lease *operationLease) Finish() {
	if lease == nil || lease.finished || lease.limiter == nil {
		return
	}
	lease.finished = true
	lease.limiter.mu.Lock()
	delete(lease.limiter.active, lease.policy.Class)
	if lease.started {
		lease.limiter.last[lease.policy.Class] = time.Now()
	}
	lease.limiter.mu.Unlock()
}

func writeRateLimited(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	http.Error(w, "operation temporarily rate limited", http.StatusTooManyRequests)
}

func (a *app) beginLimitedOperation(w http.ResponseWriter, path string) (func(), bool) {
	if a.operationLimiter == nil {
		return func() {}, true
	}
	lease, wait, ok := a.operationLimiter.acquire(path)
	if !ok {
		writeRateLimited(w, wait)
		return nil, false
	}
	lease.Start()
	return lease.Finish, true
}

package main

import (
	"fmt"
	"hash/crc32"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var providerNameSafe = regexp.MustCompile(`[^a-z0-9._-]+`)

func (a *app) providerBackoffDir() string { return filepath.Join(a.cacheDir, "provider-backoff") }
func providerBackoffName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = providerNameSafe.ReplaceAllString(name, "-")
	return strings.Trim(name, "-.")
}
func (a *app) providerBackoffPath(name string) string {
	safe := providerBackoffName(name)
	if safe == "" {
		safe = "unknown"
	}
	return filepath.Join(a.providerBackoffDir(), safe+".json")
}

func (a *app) providerBackoffActive(name string) (time.Time, string, int, bool) {
	raw := readHealthFile(a.providerBackoffPath(name))
	if raw == nil {
		return time.Time{}, "", 0, false
	}
	until := int64(jsonutil.Int(raw["until"], 0))
	if until <= time.Now().Unix() {
		return time.Time{}, "", 0, false
	}
	return time.Unix(until, 0), strings.TrimSpace(jsonutil.TextValue(raw["reason"])), jsonutil.Int(raw["failures"], 0), true
}

func (a *app) noteProviderBackoff(name string, err error) {
	if err == nil {
		return
	}
	path := a.providerBackoffPath(name)
	raw := readHealthFile(path)
	failures := jsonutil.Int(raw["failures"], 0) + 1
	// 30s, 60s, 120s ... capped at 15m. Deterministic jitter keeps several
	// dashboards from retrying together without needing random state.
	delay := 30 * time.Second
	for i := 1; i < failures && delay < 15*time.Minute; i++ {
		delay *= 2
	}
	if delay > 15*time.Minute {
		delay = 15 * time.Minute
	}
	jitter := time.Duration(crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s/%d", name, time.Now().Unix()/30)))%17) * time.Second
	payload := map[string]any{
		"level": "degraded", "provider": providerBackoffName(name), "failures": failures,
		"reason": truncate(err.Error(), 180), "updated": time.Now().Format(time.RFC3339),
		"until": time.Now().Add(delay + jitter).Unix(), "delaySeconds": int((delay + jitter).Seconds()),
	}
	_ = os.MkdirAll(a.providerBackoffDir(), 0755)
	_ = fileio.WriteJSON(path, payload)
}
func (a *app) clearProviderBackoff(name string) {
	path := a.providerBackoffPath(name)
	if _, err := os.Stat(path); err != nil {
		return
	}
	_ = fileio.WriteJSON(path, map[string]any{"level": "ok", "provider": providerBackoffName(name), "failures": 0, "updated": time.Now().Format(time.RFC3339), "lastSuccess": time.Now().Unix()})
}

// networkLikelyAvailable is deliberately fail-open. It returns false only
// when the kernel reports no usable non-loopback address at all; any ambiguous
// networking state still allows the real provider request to decide.
func networkLikelyAvailable() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return true
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ip, _, err := net.ParseCIDR(addr.String()); err == nil && !ip.IsLoopback() && !ip.IsUnspecified() {
				return true
			}
		}
	}
	return false
}

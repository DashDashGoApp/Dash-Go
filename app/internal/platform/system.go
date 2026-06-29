package platform

import (
	"context"
	"fmt"
	"maps"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

var reFirstDecimal = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

func CopyStatusMapForFacade(value map[string]any) map[string]any { return copyStatusMap(value) }
func copyStatusMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	maps.Copy(result, value)
	return result
}
func (s *Service) InvalidateSystemStatus() {
	s.statusMu.Lock()
	s.statusCache = nil
	s.statusAt = time.Time{}
	s.statusMu.Unlock()
}
func (s *Service) FreshSystemStatus() map[string]any {
	host, _ := os.Hostname()
	prof := map[string]any{}
	if s.profilePayload != nil {
		prof = s.profilePayload()
	}
	fonts := map[string]any{}
	if s.fontStatus != nil {
		fonts = s.fontStatus()
	}
	avail, swap := MemorySnapshotMB()
	mapsStatus := map[string]any{}
	if s.mapCacheStatus != nil {
		mapsStatus = s.mapCacheStatus()
	}
	theme := ""
	if s.currentTheme != nil {
		theme = s.currentTheme()
	}
	choices := any(nil)
	if s.fontStatusPayload != nil {
		choices = s.fontStatusPayload()
	}
	return map[string]any{"hostname": host, "terminalAccessEnabled": s.TerminalAccessEnabled(), "profile": prof["current"], "profileLabel": prof["label"], "profileDetail": prof["detail"], "theme": theme, "load": FirstLoadAvg(), "mem_avail_mb": avail, "swap_used_mb": swap, "disk_free_mb": DiskFreeMB(s.dashDir), "uptime": UptimeHuman(), "map": mapsStatus, "wifi": map[string]any{"ssid": WiFiSSID(), "signal": WiFiSignal(), "ip": PrimaryIP()}, "temp_c": CPUTempC(), "freq_mhz": CPUFreqMHz(), "throttled": ThrottledStatus(), "cached": false, "cache_age_s": 0, "goServer": true, "runtime": runtime.Version(), "fontsPresent": fonts["present"], "fontsMissing": fonts["missing"], "fontDir": fonts["dir"], "fontChoices": choices}
}
func (s *Service) SystemStatus() map[string]any {
	now := s.nowTime()
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.statusCache != nil && now.Sub(s.statusAt) < ControlStatusCacheTTL {
		result := copyStatusMap(s.statusCache)
		result["cached"] = true
		result["cache_age_s"] = int(now.Sub(s.statusAt).Seconds())
		return result
	}
	result := s.FreshSystemStatus()
	s.statusCache = copyStatusMap(result)
	s.statusAt = now
	return result
}
func FirstLoadAvg() string {
	f := strings.Fields(LoadAvg())
	if len(f) > 0 {
		return f[0]
	}
	return ""
}
func UptimeHuman() string {
	b, e := os.ReadFile("/proc/uptime")
	if e != nil {
		return ""
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return ""
	}
	sec, _ := strconv.Atoi(strings.Split(f[0], ".")[0])
	return fmt.Sprintf("%dd %dh %dm", sec/86400, (sec%86400)/3600, (sec%3600)/60)
}
func CPUTempC() any {
	if out := CommandText(5*time.Second, "vcgencmd", "measure_temp"); !strings.HasPrefix(out, "unavailable:") {
		if m := reFirstDecimal.FindStringSubmatch(out); len(m) > 1 {
			if f, e := strconv.ParseFloat(m[1], 64); e == nil {
				return f
			}
		}
	}
	if raw := strings.TrimSpace(fileio.ReadString("/sys/class/thermal/thermal_zone0/temp", "")); raw != "" {
		if n, e := strconv.Atoi(raw); e == nil && n > 1000 {
			return float64(n) / 1000.0
		}
	}
	return nil
}
func CPUFreqMHz() any {
	if raw := strings.TrimSpace(fileio.ReadString("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq", "")); raw != "" {
		if n, e := strconv.Atoi(raw); e == nil {
			return n / 1000
		}
	}
	if b, e := os.ReadFile("/proc/cpuinfo"); e == nil {
		for line := range strings.SplitSeq(string(b), "\n") {
			if strings.HasPrefix(strings.ToLower(line), "cpu mhz") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					if f, e := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); e == nil {
						return int(f)
					}
				}
			}
		}
	}
	return nil
}
func ThrottledStatus() any {
	if out := CommandText(5*time.Second, "vcgencmd", "get_throttled"); !strings.HasPrefix(out, "unavailable:") {
		if i := strings.Index(out, "="); i >= 0 {
			return strings.TrimSpace(out[i+1:])
		}
	}
	return nil
}
func WiFiSSID() any {
	out := CommandText(3*time.Second, "iwgetid", "-r")
	if strings.HasPrefix(out, "unavailable:") || out == "" {
		return nil
	}
	return out
}
func WiFiSignal() any {
	b, err := os.ReadFile("/proc/net/wireless")
	if err != nil {
		return nil
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if !strings.Contains(line, ":") || !strings.Contains(line, "wlan") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) <= 2 {
			continue
		}
		if quality, err := strconv.ParseFloat(strings.TrimRight(fields[2], "."), 64); err == nil {
			return int(quality / 70.0 * 100.0)
		}
	}
	return nil
}
func LoadAvg() string {
	b, e := os.ReadFile("/proc/loadavg")
	if e != nil {
		return ""
	}
	f := strings.Fields(string(b))
	if len(f) >= 3 {
		return strings.Join(f[:3], " ")
	}
	return strings.TrimSpace(string(b))
}
func DiskFreeMB(path string) int {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil || st.Bsize <= 0 {
		return 0
	}
	return int((st.Bavail * uint64(st.Bsize)) / 1024 / 1024)
}

func PrimaryIP() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return ""
}
func CommandText(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, e := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "unavailable: command timed out"
	}
	if e != nil && len(out) == 0 {
		return "unavailable: " + e.Error()
	}
	txt := strings.TrimSpace(string(out))
	if len(txt) > 12000 {
		txt = txt[len(txt)-12000:]
	}
	return txt
}
func MemoryStatus() map[string]any {
	return map[string]any{"ok": true, "capturedAt": time.Now().Unix(), "free": CommandText(5*time.Second, "free", "-h"), "swap": CommandText(5*time.Second, "swapon", "--show"), "vmstat": CommandText(6*time.Second, "vmstat", "1", "3"), "top": CommandText(8*time.Second, "bash", "-lc", "ps -eo pid,comm,rss,%mem,args --sort=-rss | head -25"), "tree": CommandText(8*time.Second, "bash", "-lc", "ps -ef | grep -E 'surf|WebKit|dashboard-control-server|control-server|Xorg|lightdm|openbox|gvfs|at-spi' | grep -v grep"), "cache": CommandText(8*time.Second, "bash", "-lc", "du -sh ~/.cache/surf ~/.local/share/webkit* ~/.cache/webkit* ~/dashboard/cache ~/dashboard/logs 2>/dev/null")}
}
func MemorySnapshotMB() (availableMB, swapUsedMB int) {
	b, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		return 0, 0
	}
	return ParseMemoryInfoMB(string(b))
}
func ParseMemoryInfoMB(contents string) (availableMB, swapUsedMB int) {
	var total, free int
	for line := range strings.SplitSeq(contents, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, e := strconv.Atoi(f[1])
		if e != nil {
			continue
		}
		switch f[0] {
		case "MemAvailable:":
			availableMB = v / 1024
		case "SwapTotal:":
			total = v
		case "SwapFree:":
			free = v
		}
	}
	if total > free {
		swapUsedMB = (total - free) / 1024
	}
	return
}

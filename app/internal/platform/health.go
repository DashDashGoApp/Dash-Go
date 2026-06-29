package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

type HealthFact struct {
	Name      string `json:"name"`
	Tier      string `json:"tier"`
	Level     string `json:"level"`
	Reason    string `json:"reason,omitempty"`
	Updated   string `json:"updated,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

func HealthRank(level string) int {
	switch strings.ToLower(level) {
	case "failing", "fail", "error":
		return 4
	case "recovering":
		return 3
	case "degraded", "warn", "warning":
		return 2
	case "unknown":
		return 1
	default:
		return 0
	}
}
func HealthLevel(rank int) string {
	switch rank {
	case 4:
		return "failing"
	case 3:
		return "recovering"
	case 2:
		return "degraded"
	case 1:
		return "unknown"
	default:
		return "ok"
	}
}
func ReadHealthFile(path string) map[string]any {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	return v
}

func HealthFactFromState(name, tier string, raw map[string]any) HealthFact {
	if raw == nil {
		return HealthFact{Name: name, Tier: tier, Level: "unknown"}
	}
	level := strings.ToLower(strings.TrimSpace(jsonutil.TextValue(raw["level"])))
	if level == "" {
		level = strings.ToLower(strings.TrimSpace(jsonutil.TextValue(raw["state"])))
	}
	switch level {
	case "healthy", "success", "ok":
		level = "ok"
	case "warning", "warn", "watch":
		level = "degraded"
	case "failed", "fail", "error":
		level = "failing"
	case "recovered", "running":
		level = "ok"
	}
	if level == "" {
		level = "unknown"
	}
	return HealthFact{Name: name, Tier: tier, Level: level, Reason: strings.TrimSpace(jsonutil.TextValue(raw["reason"])), Updated: strings.TrimSpace(jsonutil.TextValue(raw["updated"])), Timestamp: int64(jsonutil.Int(raw["timestamp"], 0))}
}
func (s *Service) clockConfirmedAt() int64 {
	return int64(jsonutil.Int(ReadHealthFile(filepath.Join(s.cacheDir, "clock-confirmed.json"))["confirmedAt"], 0))
}
func (s *Service) clockFloorEpoch() int64 {
	manifest := ReadHealthFile(filepath.Join(s.dashDir, "manifest.json"))
	if epoch := int64(jsonutil.Int(manifest["buildEpoch"], 0)); epoch > 86400 {
		return epoch - 86400
	}
	if st, e := os.Stat(filepath.Join(s.dashDir, "VERSION")); e == nil {
		epoch := st.ModTime().Unix()
		if epoch > 86400 {
			return epoch - 86400
		}
	}
	return 0
}
func (s *Service) clockTrustworthy(now time.Time) bool {
	confirmed := s.clockConfirmedAt()
	if confirmed > 0 && now.Unix() >= confirmed {
		return true
	}
	floor := s.clockFloorEpoch()
	return floor > 0 && now.Unix() >= floor
}

func (s *Service) cronEventIntervalMin(settings map[string]any) int {
	profile := strings.ToLower(jsonutil.StringValue(settings["profile"]))
	if profile == "" {
		profile = strings.ToLower(strings.TrimSpace(s.getString("profile", "balanced")))
	}
	if profile == "lite" || profile == "zero2" || profile == "low" {
		return 20
	}
	return 10
}
func ParseClockMinutes(raw any) (int, bool) {
	parts := strings.Split(jsonutil.StringValue(raw), ":")
	if len(parts) != 2 {
		return 0, false
	}
	var h, m int
	if _, e := fmt.Sscanf(parts[0], "%d", &h); e != nil {
		return 0, false
	}
	if _, e := fmt.Sscanf(parts[1], "%d", &m); e != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
func (s *Service) displaySleepState(now time.Time, settings map[string]any) (bool, int) {
	if settings["displaySleepEnabled"] == false {
		return false, -1
	}
	off, okOff := ParseClockMinutes(settings["displaySleepOff"])
	on, okOn := ParseClockMinutes(settings["displaySleepOn"])
	if !okOff || !okOn || off == on {
		return false, -1
	}
	minute := now.Hour()*60 + now.Minute()
	asleep := false
	if off < on {
		asleep = minute >= off && minute < on
	} else {
		asleep = minute >= off || minute < on
	}
	if asleep {
		return true, 0
	}
	wake := time.Date(now.Year(), now.Month(), now.Day(), on/60, on%60, 0, 0, now.Location())
	if now.Before(wake) {
		wake = wake.AddDate(0, 0, -1)
	}
	return false, int(now.Sub(wake).Minutes())
}
func (s *Service) SuppressDataStaleness(now time.Time, settings map[string]any, refresh int) bool {
	asleep, sinceWake := s.displaySleepState(now, settings)
	return asleep || (sinceWake >= 0 && sinceWake < refresh*2)
}
func CacheEpochMillis(path, key string) int64 {
	raw := ReadHealthFile(path)
	if raw == nil {
		return 0
	}
	v := int64(jsonutil.Int(raw[key], 0))
	if v > 0 && v < 100000000000 {
		v *= 1000
	}
	return v
}
func WeatherLastSuccessMillis(path string) int64 {
	raw := ReadHealthFile(path)
	if raw == nil {
		return 0
	}
	cache := jsonutil.Map(raw["cache"])
	v := int64(jsonutil.Int(cache["lastSuccessAt"], 0))
	if v > 0 && v < 100000000000 {
		v *= 1000
	}
	return v
}
func WeatherHealthEnabled(settings map[string]any) bool { return settings["weatherEnabled"] != false }
func (s *Service) DeviceHealth() map[string]any {
	nowTime := s.nowTime()
	now := nowTime.UnixMilli()
	settings := s.getSettings()
	facts := []HealthFact{}
	add := func(f HealthFact) { facts = append(facts, f) }
	safe := HealthFactFromState("safeMode", "device", ReadHealthFile(filepath.Join(s.cacheDir, "safe-mode-state.json")))
	if safe.Level == "unknown" {
		safe.Level = "ok"
	}
	add(safe)
	storageRaw := ReadHealthFile(filepath.Join(s.cacheDir, "storage-wear-state.json"))
	storage := HealthFactFromState("storage", "device", storageRaw)
	if StorageKernelWarningPredatesBoot(storageRaw, storage, BootEpochMillis(nowTime)) {
		storage.Level, storage.Reason = "ok", ""
	}
	add(storage)
	clock := HealthFact{Name: "clock", Tier: "device", Level: "ok"}
	if _, e := os.Stat(filepath.Join(s.cacheDir, "clock-unverified")); e == nil && !s.clockTrustworthy(nowTime) {
		clock.Level, clock.Reason = "degraded", "device clock has not yet been verified by network time"
	}
	add(clock)
	configFile := ""
	if s.configRevertFile != nil {
		configFile = s.configRevertFile()
	}
	cfg := HealthFactFromState("config", "device", ReadHealthFile(configFile))
	if cfg.Level == "unknown" {
		cfg.Level = "ok"
	}
	if cfg.Level == "reverted" {
		cfg.Level = "degraded"
		if cfg.Reason == "" {
			cfg.Reason = "settings reverted to last-good"
		}
	}
	add(cfg)
	updatePath := filepath.Join(s.cacheDir, "update-status.json")
	update := HealthFactFromState("update", "device", ReadHealthFile(updatePath))
	if update.Level == "unknown" {
		update.Level = "ok"
	}
	if raw := ReadHealthFile(updatePath); raw != nil && raw["rolledBack"] == true {
		update.Level = "degraded"
		if update.Reason == "" {
			update.Reason = "last update was rolled back after a health check"
		}
	}
	add(update)
	postPath := filepath.Join(s.cacheDir, "post-update-verify.json")
	post := HealthFactFromState("postUpdate", "device", ReadHealthFile(postPath))
	if post.Level == "unknown" {
		post.Level = "ok"
	}
	if raw := ReadHealthFile(postPath); raw != nil {
		switch strings.ToLower(strings.TrimSpace(jsonutil.TextValue(raw["state"]))) {
		case "pending", "running", "recovering":
			post.Level = "ok"
		case "failed", "fail", "error":
			post.Level = "failing"
			if post.Reason == "" {
				post.Reason = "a new release failed its post-update health check"
			}
		}
	}
	add(post)
	add(HealthFactFromState("healthGuard", "device", ReadHealthFile(filepath.Join(s.cacheDir, "health-guard-status.json"))))
	weatherAt := WeatherLastSuccessMillis(filepath.Join(s.cacheDir, "weather-cache.json"))
	eventsAt := CacheEpochMillis(filepath.Join(s.cacheDir, ".events-cache.meta.json"), "lastSuccessAt")
	messagesAt := CacheEpochMillis(filepath.Join(s.configDir, "message-cache.json"), "lastSuccessAt")
	dataFact := func(name string, at, fresh int64) HealthFact {
		f := HealthFact{Name: name, Tier: "data", Level: "unknown", Timestamp: at}
		if at <= 0 {
			return f
		}
		f.Level = "ok"
		if s.SuppressDataStaleness(nowTime, settings, int(fresh/60000)) {
			return f
		}
		age := now - at
		if age > fresh*8 {
			f.Level, f.Reason = "failing", "last successful data is much older than its refresh interval"
		} else if age > fresh*4 {
			f.Level, f.Reason = "degraded", "data is older than expected"
		}
		return f
	}
	if WeatherHealthEnabled(settings) {
		add(dataFact("weather", weatherAt, int64(s.weatherMinutes())*60000))
	}
	if s.isCalendarHealthEnabled() {
		add(dataFact("calendar", eventsAt, int64(s.cronEventIntervalMin(settings))*60000))
	}
	if s.isMessagesHealthEnabled() {
		add(dataFact("messages", messagesAt, 36*60*60*1000))
	}
	if entries, e := os.ReadDir(filepath.Join(s.cacheDir, "provider-backoff")); e == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			f := HealthFactFromState("provider:"+strings.TrimSuffix(entry.Name(), ".json"), "diagnostic", ReadHealthFile(filepath.Join(s.cacheDir, "provider-backoff", entry.Name())))
			if f.Level == "unknown" {
				continue
			}
			if f.Level == "recovering" {
				f.Level = "degraded"
			}
			add(f)
		}
	}
	deviceRank, dataRank := 0, 0
	deviceKnown, dataKnown := false, false
	for _, fact := range facts {
		if fact.Tier == "device" && fact.Level != "unknown" {
			deviceKnown = true
			if fact.Level != "recovering" && HealthRank(fact.Level) > deviceRank {
				deviceRank = HealthRank(fact.Level)
			}
		}
		if fact.Tier == "data" && fact.Level != "unknown" {
			dataKnown = true
			if fact.Level != "recovering" && HealthRank(fact.Level) > dataRank {
				dataRank = HealthRank(fact.Level)
			}
		}
	}
	slices.SortFunc(facts, func(left, right HealthFact) int { return strings.Compare(left.Name, right.Name) })
	device, data := HealthLevel(deviceRank), HealthLevel(dataRank)
	if !deviceKnown {
		device = "unknown"
	}
	if !dataKnown {
		data = "unknown"
	}
	line := "All systems normal"
	for _, fact := range facts {
		if fact.Tier != "device" || HealthRank(fact.Level) < 2 || fact.Level == "recovering" {
			continue
		}
		if reason := strings.TrimSpace(fact.Reason); reason != "" {
			line = reason
		} else {
			line = fact.Name + " is " + fact.Level
		}
		break
	}
	if strings.TrimSpace(line) == "" {
		line = "All systems normal"
	}
	return map[string]any{"updated": nowTime.Format(time.RFC3339), "device": device, "data": data, "statusLine": line, "facts": facts, "warningSilences": s.WarningSilences(nowTime)}
}

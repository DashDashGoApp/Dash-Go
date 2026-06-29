package platform

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

type WarningSilence struct {
	Until int64 `json:"until"`
}
type WarningSilenceState struct {
	Schema    int                       `json:"schema"`
	UpdatedAt int64                     `json:"updatedAt"`
	Data      map[string]WarningSilence `json:"data"`
}

var allowedWarningSilenceKeys = map[string]bool{"calendar": true, "weather": true, "messages": true, "storage": true, "clock": true, "config": true, "update": true, "postUpdate": true, "healthGuard": true}
var allowedWarningSilenceMinutes = map[int]bool{15: true, 60: true, 240: true, 720: true, 1440: true}

func WarningSilenceKeyAllowed(key string) bool {
	return allowedWarningSilenceKeys[strings.TrimSpace(key)]
}
func WarningSilenceMinutesAllowed(minutes int) bool { return allowedWarningSilenceMinutes[minutes] }
func WarningSilenceIsDataKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "calendar", "weather", "messages":
		return true
	default:
		return false
	}
}
func (s *Service) WarningSilenceAllowedNow(key string) bool {
	key = strings.TrimSpace(key)
	if !WarningSilenceKeyAllowed(key) {
		return false
	}
	if WarningSilenceIsDataKey(key) {
		return true
	}
	facts, _ := s.DeviceHealth()["facts"].([]HealthFact)
	for _, fact := range facts {
		if fact.Name == key && fact.Tier == "device" {
			return fact.Level == "degraded"
		}
	}
	return false
}
func EmptyWarningSilenceState() WarningSilenceState {
	return WarningSilenceState{Schema: 1, Data: map[string]WarningSilence{}}
}
func (s *Service) ReadWarningSilenceState() WarningSilenceState {
	state := EmptyWarningSilenceState()
	body, e := os.ReadFile(s.WarningSilencesPath())
	if e != nil || len(body) == 0 {
		return state
	}
	var decoded WarningSilenceState
	if json.Unmarshal(body, &decoded) != nil || decoded.Schema != 1 {
		return state
	}
	if decoded.Data == nil {
		decoded.Data = map[string]WarningSilence{}
	}
	for key, record := range decoded.Data {
		if !WarningSilenceKeyAllowed(key) || record.Until <= 0 {
			delete(decoded.Data, key)
		}
	}
	return decoded
}
func ActiveWarningSilences(state WarningSilenceState, now time.Time) map[string]any {
	active := map[string]any{}
	deadline := now.UnixMilli()
	for key, record := range state.Data {
		if WarningSilenceKeyAllowed(key) && record.Until > deadline {
			active[key] = map[string]any{"until": record.Until}
		}
	}
	return active
}
func (s *Service) WarningSilences(now time.Time) map[string]any {
	return ActiveWarningSilences(s.ReadWarningSilenceState(), now)
}
func (s *Service) SetWarningSilence(key string, minutes int, now time.Time) (map[string]any, error) {
	key = strings.TrimSpace(key)
	if !WarningSilenceKeyAllowed(key) {
		return nil, errors.New("unknown warning key")
	}
	if !WarningSilenceMinutesAllowed(minutes) {
		return nil, errors.New("minutes must be 15, 60, 240, 720, or 1440")
	}
	if !s.WarningSilenceAllowedNow(key) {
		return nil, errors.New("this critical device notice cannot be temporarily silenced")
	}
	s.warningMu.Lock()
	defer s.warningMu.Unlock()
	state := s.ReadWarningSilenceState()
	if state.Data == nil {
		state.Data = map[string]WarningSilence{}
	}
	cutoff := now.UnixMilli()
	for existing, record := range state.Data {
		if !WarningSilenceKeyAllowed(existing) || record.Until <= cutoff {
			delete(state.Data, existing)
		}
	}
	state.Schema = 1
	state.UpdatedAt = cutoff
	state.Data[key] = WarningSilence{Until: now.Add(time.Duration(minutes) * time.Minute).UnixMilli()}
	if e := fileio.WriteJSON(s.WarningSilencesPath(), state); e != nil {
		return nil, e
	}
	return ActiveWarningSilences(state, now), nil
}

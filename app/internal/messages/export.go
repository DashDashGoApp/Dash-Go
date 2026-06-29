package messages

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Preferences and Definitions expose the stable catalog and persisted selection
// payloads required by the core HTTP/CLI facade.
func (s *Service) Preferences() map[string]any         { return s.messagePrefs() }
func (s *Service) NormalizeEnabled(values []any) []any { return s.normalizeMessageEnabled(values) }

func (s *Service) CachePath() string                      { return s.messageCachePath() }
func (s *Service) OverridesPath() string                  { return s.messageOverridesPath() }
func (s *Service) SaveSourcesStatus(prefs map[string]any) { s.saveMessageSourcesStatus(prefs) }
func (s *Service) SaveCache(cache map[string]any)         { s.saveMessageCache(cache) }
func (s *Service) SaveOverrides(overrides map[string]any) { s.saveMessageOverrides(overrides) }
func (s *Service) CachePayload() map[string]any           { return s.messageCachePayload() }
func (s *Service) SourcesStatus() map[string]any          { return s.messageSourcesStatus() }
func (s *Service) DeleteItem(id string) error             { return s.deleteMessageItem(id) }
func (s *Service) ComplimentsPath() string                { return s.complimentsPath() }
func (s *Service) ComplimentsPayload() map[string]any     { return s.complimentsPayload() }

func (s *Service) LoadBirthdays() []any    { return s.loadBirthdays() }
func (s *Service) LoadCelebrations() []any { return s.loadCelebrations() }
func NextNumericID(items []any) int        { return nextNumericID(items) }
func ApplyOverrides(items []any, overrides map[string]any) []any {
	return applyMessageOverrides(items, overrides)
}

func CleanCompliment(body, existing map[string]any) (map[string]any, error) {
	return cleanCompliment(body, existing)
}
func NowMillis() int64 { return nowMillis() }

func (s *Service) Refresh(ctx context.Context, includeNetwork, manual bool) map[string]any {
	return s.refreshMessages(ctx, includeNetwork, manual)
}

func (s *Service) HandleMessages(w http.ResponseWriter, path string, body map[string]any) {
	s.handleMessages(w, path, body)
}
func (s *Service) RunSourcesCLI(args []string) int { return s.runMessageSourcesCLI(args) }
func (s *Service) HandleCompliments(w http.ResponseWriter, path string, body map[string]any) {
	s.handleCompliments(w, path, body)
}
func (s *Service) HandleSpecialDates(w http.ResponseWriter, path string, body map[string]any) {
	s.handleSpecialDates(w, path, body)
}
func (s *Service) TemporaryMessages() []any {
	return jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "temp-messages.json"), []any{}))
}
func (s *Service) ScheduledMessages() []any {
	return jsonutil.List(s.readJSONDefault(filepath.Join(s.configDir, "scheduled-messages.json"), []any{}))
}

// CanonicalCompliments owns the legacy-format normalization used by the CLI.
// It retains the current on-disk shape and equality behavior while keeping the
// migration helper next to the compliments model it normalizes.
func CanonicalCompliments(raw map[string]any) (map[string]any, bool) {
	items := jsonutil.List(raw["messages"])
	if len(items) == 0 {
		items = jsonutil.List(raw["items"])
	}
	seen := map[string]bool{}
	messages := []any{}
	for _, item := range items {
		row := jsonutil.Map(item)
		text := jsonutil.StringValue(row["text"])
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		entry := map[string]any{"text": text, "weight": clamp(jsonutil.Int(row["weight"], 1), 1, 10000)}
		if date := jsonutil.StringValue(row["date"]); date != "" {
			entry["date"] = date
		}
		messages = append(messages, entry)
	}
	canonical := map[string]any{
		"messages":        messages,
		"defaultsCleared": jsonutil.Truthy(raw["defaultsCleared"]),
		"defaultsSeeded":  jsonutil.Truthy(raw["defaultsSeeded"]),
		"removedDefaults": jsonutil.List(raw["removedDefaults"]),
		"defaultEdits":    jsonutil.Map(raw["defaultEdits"]),
		"version":         float64(4),
	}
	old, _ := json.Marshal(raw)
	newValue, _ := json.Marshal(canonical)
	return canonical, string(old) != string(newValue)
}

// Truncate preserves the established short diagnostic reason helper for core
// callers that intentionally remain outside the Messages boundary.
func Truncate(text string, limit int) string { return truncate(text, limit) }

// DecodeJSON preserves the bounded provider-payload decoder for existing
// core hardening tests and any transition-period caller.
func DecodeJSON(body io.Reader, limit int64, dst any) error {
	return decodeMessageJSON(body, limit, dst)
}

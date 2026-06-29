package main

import (
	"context"
	"io"
	"net/http"

	messagespkg "github.com/DashDashGoApp/Dash-Go/app/internal/messages"
)

// Messages now live in internal/messages. Core retains this narrow facade so
// existing HTTP routes, CLI entry points, diagnostics, demo tooling, and
// integration tests keep their stable contracts while the service owns the
// catalog, cache/override transaction lock, provider refreshes, compliments,
// and special-date persistence.
func (a *app) messagesService() *messagespkg.Service {
	a.messagesInitMu.Lock()
	defer a.messagesInitMu.Unlock()
	if a.messages == nil {
		a.messages = messagespkg.New(messagespkg.ServiceConfig{
			Home:                     a.home,
			ConfigDir:                a.configDir,
			ConfigLocal:              a.configLocal,
			CelebrationsFile:         a.celebrationsFile,
			GenerateDefaultCalendars: a.generateDefaultCalendars,
			ProviderBackoffActive:    a.providerBackoffActive,
			NoteProviderBackoff:      a.noteProviderBackoff,
			ClearProviderBackoff:     a.clearProviderBackoff,
			NetworkLikelyAvailable:   networkLikelyAvailable,
		})
	}
	return a.messages
}

func (a *app) messagePrefs() map[string]any { return a.messagesService().Preferences() }
func (a *app) normalizeMessageEnabled(values []any) []any {
	return a.messagesService().NormalizeEnabled(values)
}

func (a *app) messageCachePath() string     { return a.messagesService().CachePath() }
func (a *app) messageOverridesPath() string { return a.messagesService().OverridesPath() }
func (a *app) saveMessageSourcesStatus(prefs map[string]any) {
	a.messagesService().SaveSourcesStatus(prefs)
}
func (a *app) saveMessageCache(cache map[string]any) { a.messagesService().SaveCache(cache) }
func (a *app) saveMessageOverrides(overrides map[string]any) {
	a.messagesService().SaveOverrides(overrides)
}

func (a *app) messageSourcesStatus() map[string]any { return a.messagesService().SourcesStatus() }
func (a *app) refreshMessages(ctx context.Context, includeNetwork, manual bool) map[string]any {
	return a.messagesService().Refresh(ctx, includeNetwork, manual)
}
func (a *app) deleteMessageItem(id string) error { return a.messagesService().DeleteItem(id) }

func (a *app) handleMessages(w http.ResponseWriter, path string, body map[string]any) {
	a.messagesService().HandleMessages(w, path, body)
}
func (a *app) runMessageSourcesCLI(args []string) int { return a.messagesService().RunSourcesCLI(args) }

func (a *app) complimentsPath() string            { return a.messagesService().ComplimentsPath() }
func (a *app) complimentsPayload() map[string]any { return a.messagesService().ComplimentsPayload() }

func (a *app) handleCompliments(w http.ResponseWriter, path string, body map[string]any) {
	a.messagesService().HandleCompliments(w, path, body)
}
func (a *app) temporaryMessages() []any { return a.messagesService().TemporaryMessages() }
func (a *app) scheduledMessages() []any { return a.messagesService().ScheduledMessages() }
func (a *app) loadBirthdays() []any     { return a.messagesService().LoadBirthdays() }
func (a *app) loadCelebrations() []any  { return a.messagesService().LoadCelebrations() }
func (a *app) handleSpecialDates(w http.ResponseWriter, path string, body map[string]any) {
	a.messagesService().HandleSpecialDates(w, path, body)
}

func applyMessageOverrides(items []any, overrides map[string]any) []any {
	return messagespkg.ApplyOverrides(items, overrides)
}
func cleanCompliment(body, existing map[string]any) (map[string]any, error) {
	return messagespkg.CleanCompliment(body, existing)
}
func nextNumericID(items []any) int { return messagespkg.NextNumericID(items) }
func nowMillis() int64              { return messagespkg.NowMillis() }

// These two narrow seams keep the existing core diagnostic and hardening
// callers stable while their bounded implementation lives in internal/messages.
func truncate(s string, n int) string { return messagespkg.Truncate(s, n) }
func decodeMessageJSON(body io.Reader, limit int64, dst any) error {
	return messagespkg.DecodeJSON(body, limit, dst)
}

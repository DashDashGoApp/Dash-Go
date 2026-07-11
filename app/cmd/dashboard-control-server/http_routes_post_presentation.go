package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// handlePresentationPost owns theme, message, and map presentation mutations.
// These are intentionally separate from privileged device operations.
func (a *app) handlePresentationPost(w http.ResponseWriter, path string, body map[string]any) bool {
	switch path {
	case "/api/theme":
		theme := a.themeNameFromBody(body)
		if theme == "" {
			a.err(w, "theme name required", 400)
			return true
		}
		if ok, reason := a.themeIsAvailable(theme); !ok {
			a.err(w, reason, 400)
			return true
		}
		if err := a.writeTheme(theme); err != nil {
			a.err(w, "could not write theme: "+err.Error(), 500)
			return true
		}
		a.json(w, map[string]any{"ok": true, "theme": theme})
	case "/api/seasonal":
		enabled := jsonutil.Truthy(body["enabled"])
		if err := a.setSeasonalThemesEnabled(enabled); err != nil {
			a.err(w, "could not update seasonal rotation: "+err.Error(), 500)
			return true
		}
		a.json(w, map[string]any{"ok": true, "seasonal": enabled})
	case "/api/theme/base":
		name := jsonutil.BodyString(body, "name")
		if name == "" {
			a.err(w, "theme name required", 400)
			return true
		}
		if ok, reason := a.themeIsAvailable(name); !ok {
			a.err(w, reason, 400)
			return true
		}
		if err := os.WriteFile(filepath.Join(a.home, ".dashboard-base-theme"), []byte(name+"\n"), 0644); err != nil {
			// Preserve the existing successful response contract while making a
			// user-visible preference persistence failure diagnosable.
			log.Printf("could not persist base theme: %v", err)
		}
		a.json(w, map[string]any{"ok": true, "base": name})
	case "/api/compliments/add", "/api/compliments/delete", "/api/compliments/import", "/api/compliments/update", "/api/compliments/defaults/toggle", "/api/compliments/defaults/remove-all", "/api/compliments/defaults/add-all", "/api/compliments/clear-defaults", "/api/compliments/restore-defaults", "/api/compliments/reconcile-defaults":
		a.handleCompliments(w, path, body)
	case "/api/message-sources", "/api/message-sources/refresh", "/api/message-sources/item/delete", "/api/message-sources/item/update", "/api/temporary-messages/add", "/api/temporary-messages/delete", "/api/scheduled-messages/add", "/api/scheduled-messages/update", "/api/scheduled-messages/delete":
		a.handleMessages(w, path, body)
	case "/api/maps/prewarm":
		a.json(w, a.startMapPrewarm(body))
	case "/api/maps/cleanup":
		a.json(w, map[string]any{"ok": true, "cache": a.cleanMapImageCache(), "tileCache": a.cleanMapTileCache()})
	case "/api/maps/clear":
		a.json(w, a.clearMapCache(jsonutil.Truthy(body["clearGeocodes"]), jsonutil.Truthy(body["clearProvider"]), jsonutil.Truthy(body["clearTiles"])))
	default:
		return false
	}
	return true
}

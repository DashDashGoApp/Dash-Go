package main

import (
	"path/filepath"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// updateProgress is deliberately lighter than updateStatus. The Control card
// calls it only while an update is live, so it reads the two transaction state
// records and does not re-check the catalog, updater service, backup directory,
// backup inventory, or update log on every poll.
func (a *app) updateProgress() map[string]any {
	job := a.readUpdateJob()
	runner := jsonutil.Map(a.readJSONDefault(filepath.Join(a.cacheDir, "update-status.json"), map[string]any{}))
	state := firstNonEmpty(strOr(job["state"], ""), strOr(runner["state"], ""))
	label := firstNonEmpty(strOr(job["label"], ""), strOr(runner["label"], ""))
	detail := firstNonEmpty(strOr(job["detail"], ""), strOr(runner["detail"], ""))
	return map[string]any{
		"schema":     1,
		"capturedAt": time.Now().Unix(),
		"job":        job,
		"state":      state,
		"label":      label,
		"detail":     detail,
		"source":     firstNonEmpty(strOr(job["source"], ""), strOr(runner["source"], "")),
		"target":     firstNonEmpty(strOr(job["target"], ""), strOr(runner["target"], "")),
		"track":      firstNonEmpty(strOr(job["track"], ""), strOr(runner["track"], "")),
		"version":    firstNonEmpty(strOr(job["version"], ""), strOr(runner["version"], "")),
		"backup":     strOr(job["backup"], ""),
		"active":     updateStateActive(state),
		"terminal":   state != "" && !updateStateActive(state),
	}
}

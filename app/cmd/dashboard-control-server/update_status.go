package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) updateStatus() map[string]any {
	return a.updateStatusWithPreflight(a.updatePreflight())
}

// updateStatusFresh intentionally bypasses the short catalog cache after an
// explicit Dashboard Control "Check for updates" action. Privileged updater
// setup remains separate from this read-only selected-track catalog check.
func (a *app) updateStatusFresh() map[string]any {
	return a.updateStatusWithPreflight(a.updatePreflightFresh())
}

func (a *app) updateStatusWithPreflight(preflight map[string]any) map[string]any {
	v := fileio.ReadString(filepath.Join(a.dash, "VERSION"), "")
	backups := a.listConfigBackups()
	keep := a.configBackupKeepLimit()
	totalSize := int64(0)
	autoCount := 0
	preAction := []map[string]any{}
	for _, b := range backups {
		totalSize += int64(jsonutil.Int(b["size"], 0))
		kind := strOr(b["kind"], "")
		if strings.HasPrefix(kind, "pre-") || strOr(b["preAction"], "") != "" {
			preAction = append(preAction, b)
		}
		if kind != "manual" && kind != "" {
			autoCount++
		}
	}
	latest := any(nil)
	if len(backups) > 0 {
		latest = backups[0]
	}
	logPath := filepath.Join(a.logDir, "update.log")
	statusPath := filepath.Join(a.cacheDir, "update-status.json")
	installerStatus := jsonutil.Map(a.readJSONDefault(statusPath, map[string]any{}))
	job := a.readUpdateJob()
	state := firstNonEmpty(strOr(job["state"], ""), strOr(installerStatus["state"], ""), a.updateLogState())
	label := firstNonEmpty(strOr(job["label"], ""), strOr(installerStatus["label"], ""), a.updateLogLabel())
	detail := firstNonEmpty(strOr(job["detail"], ""), strOr(installerStatus["detail"], ""), tailFile(logPath, 500))
	return map[string]any{
		"installedVersion": v, "manifestVersion": manifestVersion(a.dash), "versionMismatch": false,
		"installerPresent": preflight["installerPresent"], "updateTrackProfilePresent": preflight["updateTrackProfilePresent"], "updateReady": preflight["ready"],
		"preflight": preflight, "job": job, "updaterUnit": preflight["unit"],
		"restoreAvailable": len(backups) > 0, "rollbackSupported": len(backups) > 0,
		"backups": backups, "backupCount": len(backups), "backupAutomaticCount": autoCount, "preActionBackups": preAction,
		"backupDir": a.backupDir(), "backupKeep": keep, "backupOverLimit": len(backups) > keep, "backupPruneAvailable": len(backups) > keep,
		"backupTotalSize": totalSize, "latestBackup": latest, "availability": preflight["availability"], "problems": preflight["problems"],
		"updateLogMtime": fileMod(logPath), "updateLogSize": fileSize(logPath), "updateLogState": state, "updateLogLabel": label,
		"updateLogDetail": detail, "updateLogSource": firstNonEmpty(strOr(job["source"], ""), strOr(installerStatus["source"], "")),
		"updateLogTarget":  firstNonEmpty(strOr(job["target"], ""), strOr(installerStatus["target"], "")),
		"updateLogTrack":   firstNonEmpty(strOr(job["track"], ""), strOr(installerStatus["track"], "")),
		"updateLogVersion": firstNonEmpty(strOr(job["version"], ""), strOr(installerStatus["version"], "")),
	}
}
func manifestVersion(dash string) string {
	m := jsonutil.Map((&app{dash: dash}).readJSONDefault(filepath.Join(dash, "manifest.json"), map[string]any{}))
	return strOr(m["version"], "")
}

func fileMod(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.ModTime().Unix()
}

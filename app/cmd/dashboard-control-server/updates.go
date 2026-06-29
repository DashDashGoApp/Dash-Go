package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) loadHealthStatus() map[string]any {
	def := map[string]any{"exists": false, "state": "unknown", "label": "Not checked yet", "passCount": 0, "fixCount": 0, "warnCount": 0, "failCount": 0, "issueCount": 0, "issues": []any{}, "sections": []any{}, "checkedAt": 0, "ageSeconds": nil, "outputTail": ""}
	v := a.readJSONDefault(filepath.Join(a.cacheDir, "health-status.json"), def)
	if m, ok := v.(map[string]any); ok {
		m["exists"] = true
		m["ageSeconds"] = time.Now().Unix() - int64(jsonutil.Int(m["checkedAt"], 0))
		return m
	}
	return def
}

const systemUpdateInterruptedGrace = 2 * time.Minute

func systemUpdateStateActive(state string) bool {
	switch state {
	case "starting", "running", "updating", "upgrading":
		return true
	default:
		return false
	}
}

func systemBootID() string {
	return strings.TrimSpace(fileio.ReadString("/proc/sys/kernel/random/boot_id", ""))
}

func (a *app) recoverInterruptedSystemUpdateStatus(m map[string]any, now time.Time, detail string) {
	m["state"] = "failed"
	m["label"] = "System update interrupted"
	m["detail"] = detail
	m["interrupted"] = true
	m["recoveredAt"] = now.Unix()
	m["updatedAt"] = now.Unix()
	a.writeSystemUpdateStatus(m)
}

func (a *app) systemUpdateStatus() map[string]any {
	path := filepath.Join(a.cacheDir, "system-update-status.json")
	def := map[string]any{"state": "never", "running": false, "label": "Not run yet", "detail": "Run a package update from Dashboard Control when you are ready."}
	v := a.readJSONDefault(path, def)
	m, ok := v.(map[string]any)
	if !ok {
		m = def
	}
	now := time.Now()
	state := strOr(m["state"], "never")
	activeState := systemUpdateStateActive(state)
	lockDir := filepath.Join(a.cacheDir, "system-update.lock")
	lockInfo, lockErr := os.Stat(lockDir)
	lockPresent := lockErr == nil && lockInfo.IsDir()
	lockPid := readIntFile(filepath.Join(lockDir, "pid"))
	commandPid := jsonutil.Int(m["commandPid"], 0)
	lockRunning := pidRunning(lockPid)
	commandRunning := pidRunning(commandPid)
	currentBootID := systemBootID()
	statusBootID := strOr(m["bootId"], "")
	updatedAt := int64(jsonutil.Int(m["updatedAt"], jsonutil.Int(m["startedAt"], 0)))
	lastHeartbeat := int64(jsonutil.Int(m["lastHeartbeat"], int(updatedAt)))
	statusAge := 0
	if updatedAt > 0 {
		statusAge = int(now.Unix() - updatedAt)
	}
	bootChanged := activeState && statusBootID != "" && currentBootID != "" && statusBootID != currentBootID
	noLiveOwner := !lockRunning && !commandRunning
	agedOut := updatedAt > 0 && now.Sub(time.Unix(updatedAt, 0)) >= systemUpdateInterruptedGrace
	// A lock directory without a valid PID is intentionally ambiguous. Preserve
	// it rather than risking a second apt/dpkg transaction; beta.30 writes the
	// PID before starting apt so new locks are always recoverable.
	unknownLock := lockPresent && lockPid <= 0
	if activeState && (bootChanged || (noLiveOwner && agedOut && !unknownLock)) {
		detail := "The package update helper stopped before recording a final result. Check the system update log, then retry when ready."
		if bootChanged {
			detail = "The device restarted before the package update helper recorded a final result. Check the system update log, then retry when ready."
		}
		a.recoverInterruptedSystemUpdateStatus(m, now, detail)
		state = "failed"
		activeState = false
		updatedAt = now.Unix()
		lastHeartbeat = now.Unix()
		statusAge = 0
	}
	m["lockPresent"] = lockPresent
	m["lockPid"] = lockPid
	m["lockRunning"] = lockRunning
	m["commandPid"] = commandPid
	m["commandRunning"] = commandRunning
	m["currentBootId"] = currentBootID
	m["statusAgeSeconds"] = statusAge
	m["heartbeatAgeSeconds"] = tern(lastHeartbeat > 0, int(now.Unix()-lastHeartbeat), 0)
	m["lastHeartbeat"] = lastHeartbeat
	m["running"] = activeState
	m["scriptPresent"] = fileio.Exists(filepath.Join(a.binDir, "dashboard-system-update.sh"))
	m["sudoPresent"] = lookPath("sudo")
	m["aptGetPresent"] = lookPath("apt-get")
	m["ready"] = m["scriptPresent"] == true && m["sudoPresent"] == true && m["aptGetPresent"] == true && !activeState
	m["logExists"] = fileio.Exists(filepath.Join(a.logDir, "system-update.log"))
	m["logSize"] = fileSize(filepath.Join(a.logDir, "system-update.log"))
	m["logMtime"] = fileMod(filepath.Join(a.logDir, "system-update.log"))
	m["rebootRecommended"] = fileio.Exists("/var/run/reboot-required")
	m["problems"] = []any{}
	if m["ready"] != true {
		m["hint"] = "Run installer/doctor repair if system update controls are unavailable."
	} else {
		m["hint"] = "Ready to run apt-get update/upgrade through Dashboard Control."
	}
	return m
}

func readIntFile(path string) int {
	s := strings.TrimSpace(fileio.ReadString(path, "0"))
	n, _ := strconv.Atoi(s)
	return n
}
func pidRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func fileSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}
func (a *app) writeSystemUpdateStatus(m map[string]any) {
	if err := fileio.WriteJSON(filepath.Join(a.cacheDir, "system-update-status.json"), m); err != nil {
		// Status persistence is intentionally non-blocking for the update flow,
		// but this is user-visible Control state and should never fail silently.
		log.Printf("could not persist system update status: %v", err)
	}
}
func (a *app) startSystemUpdate() (map[string]any, error) {
	st := a.systemUpdateStatus()
	if st["running"] == true {
		return nil, errors.New("system update is already running")
	}
	script := filepath.Join(a.binDir, "dashboard-system-update.sh")
	if !fileio.Exists(script) {
		return nil, errors.New("system update helper is missing")
	}
	if !lookPath("sudo") {
		return nil, errors.New("sudo is missing")
	}
	if !lookPath("apt-get") {
		return nil, errors.New("apt-get is missing")
	}
	cmdArgs := []string{script}
	detail := "Launching the package update helper…"
	prof := a.profilePayload()
	profileBase := jsonutil.StringValue(prof["base"])
	if profileBase == "" {
		profileBase = jsonutil.StringValue(prof["current"])
	}
	if profileBase == "lite" && fileio.Exists(filepath.Join(a.binDir, "dashboard-maintenance.sh")) {
		cmdArgs = []string{filepath.Join(a.binDir, "dashboard-maintenance.sh"), "system-update"}
		detail = "Launching the package update helper in low-profile maintenance mode. The kiosk browser may pause until the update finishes."
	}
	now := time.Now().Unix()
	bootID := systemBootID()
	initialStatus := map[string]any{"state": "starting", "label": "Starting system update", "detail": detail, "updatedAt": now, "startedAt": now, "bootId": bootID, "logFile": filepath.Join(a.logDir, "system-update.log"), "maintenanceMode": len(cmdArgs) > 1, "profile": prof["current"]}
	a.writeSystemUpdateStatus(initialStatus)
	cmd := exec.Command("bash", cmdArgs...)
	cmd.Dir = a.dash
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		initialStatus["state"] = "failed"
		initialStatus["label"] = "System update could not start"
		initialStatus["detail"] = "The package update helper could not start."
		initialStatus["updatedAt"] = time.Now().Unix()
		a.writeSystemUpdateStatus(initialStatus)
		return nil, err
	}
	return map[string]any{"started": true, "status": a.systemUpdateStatus()}, nil
}
func (a *app) updateLogState() string {
	p := filepath.Join(a.logDir, "update.log")
	txt := strings.ToLower(tailFile(p, 4000))
	if txt == "" {
		return "never"
	}
	if strings.Contains(txt, "update complete") || strings.Contains(txt, "success") {
		return "success"
	}
	if strings.Contains(txt, "failed") || strings.Contains(txt, "!!") {
		return "failed"
	}
	return "check"
}
func (a *app) updateLogLabel() string {
	switch a.updateLogState() {
	case "success":
		return "Success"
	case "failed":
		return "Failed"
	case "check":
		return "Check log"
	default:
		return "—"
	}
}

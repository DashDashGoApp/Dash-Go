package platform

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

var diagnosticSecretPatterns = []*regexp.Regexp{regexp.MustCompile(`(?m)^(\s*DASH_(?:[A-Z0-9_]*(?:KEY|SECRET|TOKEN|PASS|PASSWORD)|RADAR_XWEATHER_ID|CONTROL_PIN_HASH|CONTROL_PIN_SALT|TOKEN|USERPASS)\s*=\s*).*`), regexp.MustCompile(`(?im)(\bapi[_-]?key\b["']?\s*[:=]\s*["']?)[^"'\s,}\]]+`)}
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var reDoctorLine = regexp.MustCompile(`^(OK|WARN|FAIL|BAD|FIXED)\s*:?\s*(.*)$`)

func RedactText(text string) string {
	for _, re := range diagnosticSecretPatterns {
		text = re.ReplaceAllString(text, `${1}<redacted>`)
	}
	return text
}
func StripANSI(text string) string { return reANSI.ReplaceAllString(text, "") }

func tailString(value string, limit int) string {
	if len(value) > limit {
		return value[len(value)-limit:]
	}
	return value
}
func tailFile(path string, limit int) string {
	body, e := os.ReadFile(path)
	if e != nil {
		return ""
	}
	return tailString(string(body), limit)
}
func (s *Service) BuildDiagnostics() (map[string]any, error) {
	return s.BuildDiagnosticsWithHealth(s.RunDoctorSummary())
}
func (s *Service) BuildDiagnosticsWithHealth(health map[string]any) (map[string]any, error) {
	dir := s.DiagnosticsDir()
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	if e := os.Chmod(dir, 0700); e != nil {
		return nil, e
	}
	f, e := os.CreateTemp(dir, ".dashboard-diagnostics-*.zip")
	if e != nil {
		return nil, e
	}
	stage := f.Name()
	defer os.Remove(stage)
	if e := f.Chmod(0600); e != nil {
		_ = f.Close()
		return nil, e
	}
	z := zip.NewWriter(f)
	add := func(name, text string) { w, _ := z.Create(name); _, _ = w.Write([]byte(text)) }
	hb, _ := json.MarshalIndent(health, "", " ")
	cb, _ := json.MarshalIndent(s.CacheStatus(), "", " ")
	maps := map[string]any{}
	if s.mapCacheStatus != nil {
		maps = s.mapCacheStatus()
	}
	mb, _ := json.MarshalIndent(maps, "", " ")
	actions := any([]any{})
	if s.actionHistory != nil {
		actions = s.actionHistory()
	}
	ab, _ := json.MarshalIndent(actions, "", " ")
	updates := map[string]any{}
	if s.systemUpdateStatus != nil {
		updates = s.systemUpdateStatus()
	}
	sb, _ := json.MarshalIndent(updates, "", " ")
	add("doctor.txt", jsonutil.TextValue(health["outputTail"]))
	add("health-status.json", string(hb))
	add("cache-status.json", string(cb))
	add("map-status.json", string(mb))
	add("action-history.json", string(ab))
	add("system-update-status.json", string(sb))
	add("housekeeping-status.json", tailFile(filepath.Join(s.cacheDir, "housekeeping-status.json"), 20000))
	for _, item := range []struct{ name, path string }{{"logs/system-update.log", filepath.Join(s.logDir, "system-update.log")}, {"logs/housekeeping.log", filepath.Join(s.logDir, "housekeeping.log")}, {"logs/events-cache.log", filepath.Join(s.logDir, "events-cache.log")}, {"logs/update.log", filepath.Join(s.logDir, "update.log")}, {"config/settings.json", s.settingsFile}, {"config/config.local.js", s.configLocal}, {"config/dashboard-control.env", s.controlEnv}, {"config/dashboard-update.env", filepath.Join(s.homeDir, ".dashboard-update.env")}, {"config/dashboard-radar.env", filepath.Join(s.homeDir, ".dashboard-radar.env")}} {
		add(item.name, RedactText(tailFile(item.path, 40000)))
	}
	if e := z.Close(); e != nil {
		_ = f.Close()
		return nil, e
	}
	if e := f.Close(); e != nil {
		return nil, e
	}
	path := filepath.Join(dir, DiagnosticsBundleName)
	if e := os.Rename(stage, path); e != nil {
		return nil, e
	}
	if e := os.Chmod(path, 0600); e != nil {
		return nil, e
	}
	st, e := os.Stat(path)
	if e != nil {
		return nil, e
	}
	return map[string]any{"ok": health["ok"], "file": filepath.Base(path), "location": s.DiagnosticsLocationHint(), "size": st.Size(), "doctorOk": health["ok"]}, nil
}
func ParseDoctorReport(output string, rc int, dur time.Duration) map[string]any {
	txt := StripANSI(output)
	ok, warn, fail, fixed := 0, 0, 0, 0
	issues := []map[string]any{}
	section := "General"
	sections := map[string]map[string]int{}
	for raw := range strings.SplitSeq(txt, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "== ") {
			section = strings.TrimSpace(strings.TrimPrefix(line, "== "))
			if section == "" {
				section = "General"
			}
			if sections[section] == nil {
				sections[section] = map[string]int{"ok": 0, "warn": 0, "fail": 0}
			}
			continue
		}
		m := reDoctorLine.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		status := strings.ToUpper(m[1])
		level := strings.ToLower(status)
		switch status {
		case "BAD":
			level = "fail"
		case "FIXED":
			level = "ok"
			fixed++
		}
		msg := strings.TrimSpace(m[2])
		if sections[section] == nil {
			sections[section] = map[string]int{"ok": 0, "warn": 0, "fail": 0}
		}
		sections[section][level]++
		switch level {
		case "ok":
			ok++
		case "warn":
			warn++
			if len(issues) < 12 {
				issues = append(issues, map[string]any{"level": "warn", "section": section, "message": msg})
			}
		case "fail":
			fail++
			if len(issues) < 12 {
				issues = append(issues, map[string]any{"level": "fail", "section": section, "message": msg})
			}
		}
	}
	state, label := "healthy", "Healthy"
	if fail > 0 {
		state, label = "action", "Action needed"
	} else if warn > 0 {
		state, label = "check", "Check soon"
	}
	if rc != 0 && fail == 0 {
		state, label = "action", "Action needed"
		fail = 1
		issues = append([]map[string]any{{"level": "fail", "section": "Doctor", "message": fmt.Sprintf("doctor.sh exited with code %d", rc)}}, issues...)
	}
	sectionOut := []map[string]any{}
	for name, count := range sections {
		sectionOut = append(sectionOut, map[string]any{"name": name, "ok": count["ok"], "warn": count["warn"], "fail": count["fail"]})
	}
	return map[string]any{"ok": rc == 0 && fail == 0, "state": state, "label": label, "passCount": ok, "fixCount": fixed, "warnCount": warn, "failCount": fail, "issueCount": warn + fail, "issues": issues, "sections": sectionOut, "returnCode": rc, "durationSeconds": dur.Seconds(), "checkedAt": time.Now().Unix()}
}
func (s *Service) RunDoctorSummary() map[string]any { return s.RunDoctorSummaryMode(false, false) }
func (s *Service) RunDoctorSummaryMode(repair, plan bool) map[string]any {
	start := s.nowTime()
	timeout := 90 * time.Second
	if repair || plan {
		timeout = 180 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	args := []string{filepath.Join(s.binDir, "doctor.sh"), "--full", "--no-prompt"}
	if plan {
		args = append(args, "--plan")
	} else if repair {
		args = append(args, "--yes")
	}
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Env = os.Environ()
	if repair {
		cmd.Env = append(cmd.Env, "DASH_DOCTOR_FROM_API=1")
	}
	outb, err := cmd.CombinedOutput()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = 1
		}
	}
	out := StripANSI(string(outb))
	if ctx.Err() == context.DeadlineExceeded {
		rc = 124
		out += "\nhealth check timed out"
	}
	if out == "" && err != nil {
		out = "health check failed: " + err.Error()
	}
	summary := ParseDoctorReport(out, rc, s.nowTime().Sub(start))
	summary["repairMode"] = repair
	summary["planMode"] = plan
	summary["outputTail"] = tailString(out, 12000)
	_ = fileio.WriteJSON(filepath.Join(s.cacheDir, "health-status.json"), summary)
	return summary
}

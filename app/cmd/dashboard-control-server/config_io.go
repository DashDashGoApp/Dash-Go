package main

import (
	"cmp"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

func (a *app) readJSONDefault(path string, def any) any {
	var v any
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return def
	}
	return v
}
func lookPath(p string) bool { _, err := exec.LookPath(p); return err == nil }
func runCmd(name string, args ...string) int {
	c := exec.Command(name, args...)
	if err := c.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		return 1
	}
	return 0
}

func tailFile(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}
func (a *app) logPath(name string) string {
	switch name {
	case "cache":
		return filepath.Join(a.logDir, "events-cache.log")
	case "update":
		return filepath.Join(a.logDir, "update.log")
	case "system-update":
		return filepath.Join(a.logDir, "system-update.log")
	case "terminal":
		return filepath.Join(a.logDir, "terminal.log")
	default:
		return filepath.Join(a.logDir, "events-cache.log")
	}
}
func clamp[T cmp.Ordered](n, lo, hi T) T {
	return max(lo, min(hi, n))
}

package main

import (
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// runtimeReady is intentionally tiny and unauthenticated. It proves that the
// loopback Go runtime has bound its HTTP listener and is serving the installed
// payload version, without exposing Control status, settings, or credentials.
// The kiosk and installer use it as a bounded launch/update readiness gate.
func (a *app) runtimeReady() map[string]any {
	version := strings.TrimSpace(a.releaseVersion)
	if version == "" {
		version = strings.TrimSpace(fileio.ReadString(filepath.Join(a.dash, "VERSION"), ""))
	}
	return map[string]any{"goServer": true, "version": version}
}

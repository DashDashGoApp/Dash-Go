package main

import "regexp"

// These expressions are intentionally package-level. They run while rewriting
// config.local.js and naming backups; compiling them per request needlessly
// allocates and rebuilds the regexp program. Patterns whose logic moved into
// internal packages were removed here so every process start (including the
// short-lived CLI modes) no longer compiles dead programs.
var (
	reBackupFilenameSafe   = regexp.MustCompile(`[^0-9A-Za-z._-]+`)
	rePauseBeforeProfile   = regexp.MustCompile(`(?i)(\n\s*pauseWhileOpen\s*:\s*(?:true|false))\s*(\n\s*profile\s*:)`)
	rePauseProfilePair     = regexp.MustCompile(`(?i)\n\s*pauseWhileOpen\s*:\s*(?:true|false)\s*\n\s*profile\s*:`)
	reDashboardLocalObject = regexp.MustCompile(`(window\.DASHBOARD_LOCAL\s*=\s*\{\s*\n)`)
	reDemoMode             = regexp.MustCompile(`(?i)(["']?demoMode["']?\s*:\s*true\s*,?\s*)`)
)

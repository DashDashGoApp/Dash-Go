//go:build windows

package platform

import "os/exec"

// Dashboard Control terminal launch is unavailable in Windows Showcase
// packages. Keep the helper explicit so the server still compiles cleanly.
func prepareDetachedTerminalCommand(cmd *exec.Cmd) {}

//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

func prepareDetachedTerminalCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

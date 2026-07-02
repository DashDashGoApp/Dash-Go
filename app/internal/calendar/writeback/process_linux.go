//go:build linux

package writeback

import (
	"errors"
	"syscall"
)

func processGone(pid int) bool {
	err := syscall.Kill(pid, 0)
	return errors.Is(err, syscall.ESRCH)
}

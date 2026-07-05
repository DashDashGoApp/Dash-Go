//go:build windows

package main

import (
	"os"
	"os/exec"
	"sync"
)

var runtimeLocks sync.Map

func withRuntimeFileLock(path string, fn func() error) error {
	value, _ := runtimeLocks.LoadOrStore(path, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

// Windows Showcase packages intentionally disable updater process control, so
// treating an arbitrary PID as live would be unsafe. No cross-process lock is
// inferred from a PID in this runtime.
func runtimeLockHeld(path string) (bool, error) { return false, nil }

func runtimeProcessRunning(process *os.Process) bool { return false }

func prepareDetachedRuntimeCommand(cmd *exec.Cmd) {}

func runtimeTerminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

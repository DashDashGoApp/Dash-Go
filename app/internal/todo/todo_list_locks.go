package todo

import (
	"strings"
	"sync"
)

// todoListLock serializes local mutations for one list without holding the
// cloud lane across network waits. Atomic rename protects readers; this lock
// protects read-modify-write transitions among checkbox, clear, queue-settle,
// and delta-merge operations for the same list.
func (a *Service) todoListLock(listID string) func() {
	key := strings.TrimSpace(listID)
	a.todoListLocksMu.Lock()
	if a.todoListLocks == nil {
		a.todoListLocks = map[string]*sync.Mutex{}
	}
	lock := a.todoListLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		a.todoListLocks[key] = lock
	}
	a.todoListLocksMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

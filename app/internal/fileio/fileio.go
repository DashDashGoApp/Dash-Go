// Package fileio provides small, dependency-free helpers for local file access.
package fileio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// syncDirectory persists a completed rename's directory entry where the local
// filesystem supports directory fsync. The platform implementation remains
// conservative on runtimes that do not expose POSIX directory synchronization.
func syncDirectory(dir string) error {
	return syncDirectoryPlatform(dir)
}

// WriteAtomic writes b through a same-directory temporary file and atomically
// renames it into place. Data, requested permissions, and the parent directory
// entry are flushed in the durable order needed by abrupt SD-card power loss.
func WriteAtomic(path string, b []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		cleanup()
		return err
	}
	// Flush file contents and mode before the atomic replacement. Renaming a
	// dirty temporary file can otherwise expose an empty/truncated destination
	// after sudden power loss on inexpensive removable storage.
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return syncDirectory(dir)
}

// WriteJSON writes an indented JSON document with a trailing newline through a
// unique same-directory temporary path before replacing the destination.
func WriteJSON(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	// A unique same-directory temporary path keeps concurrent, unrelated JSON
	// writers from colliding on a shared `.tmp` name while retaining atomic rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	// Match WriteAtomic: content and permissions are durable before rename.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

// WriteCompactJSON writes compact JSON with a trailing newline through the
// same durable fsync-before-rename path used for other managed state.
func WriteCompactJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return WriteAtomic(path, b, 0644)
}

// Exists reports whether p is an existing non-directory path.
func Exists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ReadString returns the trimmed contents of path, or def when the path is
// missing, unreadable, or empty after trimming.
func ReadString(path, def string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return def
	}
	return s
}

// RemoveDurable deletes a managed file and flushes its parent directory entry.
// It is used for vdir event deletion so a power loss cannot resurrect a file
// after Dash-Go has already acknowledged its removal.
func RemoveDurable(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

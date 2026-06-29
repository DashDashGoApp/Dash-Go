// Package fileio provides small, dependency-free helpers for local file access.
package fileio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// WriteAtomic writes b through a same-directory temporary file and atomically
// renames it into place.
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
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	_ = os.Chmod(tmpName, mode)
	return os.Rename(tmpName, path)
}

// WriteJSON writes an indented JSON document with a trailing newline through a
// unique same-directory temporary path before replacing the destination.
func WriteJSON(path string, v any) error {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
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

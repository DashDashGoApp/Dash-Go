//go:build windows

package fileio

// Windows does not expose the POSIX directory-fsync durability step used after
// an atomic rename. The replacement file has already been synced before rename.
func syncDirectoryPlatform(dir string) error { return nil }

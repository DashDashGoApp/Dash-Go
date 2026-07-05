//go:build !windows

package platform

import "syscall"

func platformDiskFreeMB(path string) int {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil || st.Bsize <= 0 {
		return 0
	}
	return int((st.Bavail * uint64(st.Bsize)) / (1024 * 1024))
}

//go:build !linux

package writeback

func processGone(pid int) bool { return false }

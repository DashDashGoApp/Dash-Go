//go:build windows

package platform

// Windows Showcase packages never offer device-storage repair. Returning zero
// keeps the existing status surface conservative rather than inferring a drive.
func platformDiskFreeMB(path string) int { return 0 }

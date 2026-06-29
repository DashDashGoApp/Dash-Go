package main

import "testing"

func TestParseMemoryInfoMBReportsAvailableAndUsedSwap(t *testing.T) {
	available, swapUsed := parseMemoryInfoMB("MemTotal: 468992 kB\nMemAvailable: 217088 kB\nSwapTotal: 2293760 kB\nSwapFree: 2253824 kB\n")
	if available != 212 {
		t.Fatalf("available=%d, want 212", available)
	}
	if swapUsed != 39 {
		t.Fatalf("swapUsed=%d, want 39", swapUsed)
	}
}

func TestParseMemoryInfoMBDoesNotReportNegativeSwap(t *testing.T) {
	available, swapUsed := parseMemoryInfoMB("MemAvailable: 1024 kB\nSwapTotal: 1024 kB\nSwapFree: 2048 kB\n")
	if available != 1 || swapUsed != 0 {
		t.Fatalf("available=%d swapUsed=%d, want 1 0", available, swapUsed)
	}
}

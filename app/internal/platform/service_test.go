package platform

import (
	"testing"
	"time"
)

func TestParseTerminalAccessKeepsFailOpenDefault(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		enabled bool
		valid   bool
	}{
		{"enabled", "DASH_TERMINAL_ACCESS=1\n", true, true},
		{"disabled", "DASH_TERMINAL_ACCESS=0\n", false, true},
		{"empty", "", true, false},
		{"malformed", "DASH_TERMINAL_ACCESS=no\n", true, false},
		{"conflict", "DASH_TERMINAL_ACCESS=1\nDASH_TERMINAL_ACCESS=0\n", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enabled, valid := ParseTerminalAccess([]byte(tc.input))
			if enabled != tc.enabled || valid != tc.valid {
				t.Fatalf("ParseTerminalAccess(%q) = (%v, %v), want (%v, %v)", tc.input, enabled, valid, tc.enabled, tc.valid)
			}
		})
	}
}

func TestWarningSilenceRulesRemainBounded(t *testing.T) {
	if !WarningSilenceKeyAllowed("messages") || WarningSilenceKeyAllowed("device") {
		t.Fatal("warning silence keys must allow named data keys but reject generic device state")
	}
	if !WarningSilenceMinutesAllowed(15) || !WarningSilenceMinutesAllowed(1440) || WarningSilenceMinutesAllowed(30) {
		t.Fatal("warning silence durations must remain bounded to the established choices")
	}
	state := EmptyWarningSilenceState()
	state.Data["messages"] = WarningSilence{Until: time.Now().Add(time.Minute).UnixMilli()}
	if _, ok := ActiveWarningSilences(state, time.Now())["messages"]; !ok {
		t.Fatal("active silence must be returned before expiry")
	}
}

func TestParseMemoryInfoMB(t *testing.T) {
	available, swap := ParseMemoryInfoMB("MemTotal: 458000 kB\nMemAvailable: 103424 kB\nSwapTotal: 229376 kB\nSwapFree: 174080 kB\n")
	if available != 101 || swap != 54 {
		t.Fatalf("ParseMemoryInfoMB = (%d, %d), want (101, 54)", available, swap)
	}
}

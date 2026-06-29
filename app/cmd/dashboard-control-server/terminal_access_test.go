package main

import (
	"os"
	"testing"
)

func TestTerminalAccessDefaultsToEnabledAndWritesOwnerOnlyState(t *testing.T) {
	a := &app{home: t.TempDir()}
	if !a.terminalAccessEnabled() {
		t.Fatal("missing terminal-access file must preserve the enabled legacy default")
	}
	if err := a.setTerminalAccessEnabled(false); err != nil {
		t.Fatal(err)
	}
	if a.terminalAccessEnabled() {
		t.Fatal("disabled terminal-access file was not honored")
	}
	st, err := os.Stat(a.terminalAccessFile())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("terminal-access file mode = %o, want 0600", st.Mode().Perm())
	}
	if err := a.setTerminalAccessEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !a.terminalAccessEnabled() {
		t.Fatal("enabled terminal-access file was not honored")
	}
}

func TestParseTerminalAccessRejectsMalformedOrConflictingState(t *testing.T) {
	for _, tc := range []struct {
		input   string
		enabled bool
		valid   bool
	}{
		{"DASH_TERMINAL_ACCESS=1\n", true, true},
		{"DASH_TERMINAL_ACCESS=0\n", false, true},
		{"# Dash-Go\nDASH_TERMINAL_ACCESS=0\n", false, true},
		{"DASH_TERMINAL_ACCESS=maybe\n", true, false},
		{"DASH_TERMINAL_ACCESS=1\nDASH_TERMINAL_ACCESS=0\n", true, false},
	} {
		enabled, valid := parseTerminalAccess([]byte(tc.input))
		if enabled != tc.enabled || valid != tc.valid {
			t.Fatalf("parseTerminalAccess(%q) = (%v, %v), want (%v, %v)", tc.input, enabled, valid, tc.enabled, tc.valid)
		}
	}
}

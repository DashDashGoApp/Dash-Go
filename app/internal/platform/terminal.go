package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

var ErrTerminalAccessDisabled = errors.New("terminal access is disabled by the SSH administrator")

// ParseTerminalAccess keeps the existing owner-side switch format. Missing or
// malformed state remains enabled so a damaged optional file cannot silently
// remove a long-standing recovery surface from an installed dashboard.
func ParseTerminalAccess(data []byte) (enabled bool, valid bool) {
	seen := false
	enabled = true
	for raw := range strings.SplitSeq(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var next bool
		switch line {
		case "DASH_TERMINAL_ACCESS=1":
			next = true
		case "DASH_TERMINAL_ACCESS=0":
			next = false
		default:
			return true, false
		}
		if seen && enabled != next {
			return true, false
		}
		seen, enabled = true, next
	}
	return enabled, seen
}
func (s *Service) TerminalAccessEnabled() bool {
	b, e := os.ReadFile(s.TerminalAccessFile())
	if e != nil {
		return true
	}
	enabled, valid := ParseTerminalAccess(b)
	if !valid {
		return true
	}
	return enabled
}
func (s *Service) SetTerminalAccessEnabled(enabled bool) error {
	value := "DASH_TERMINAL_ACCESS=0\n"
	if enabled {
		value = "DASH_TERMINAL_ACCESS=1\n"
	}
	return fileio.WriteAtomic(s.TerminalAccessFile(), []byte(value), 0600)
}
func (s *Service) RunTerminalAccessCLI(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: dashboard-terminal-access status|enable|disable")
		return 64
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "status":
		if s.TerminalAccessEnabled() {
			fmt.Println("enabled")
		} else {
			fmt.Println("disabled")
		}
		return 0
	case "enable":
		if e := s.SetTerminalAccessEnabled(true); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		fmt.Println("terminal access enabled")
		return 0
	case "disable":
		if e := s.SetTerminalAccessEnabled(false); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		fmt.Println("terminal access disabled")
		return 0
	default:
		fmt.Fprintln(os.Stderr, "usage: dashboard-terminal-access status|enable|disable")
		return 64
	}
}
func ternary(b bool, a, c string) string {
	if b {
		return a
	}
	return c
}
func (s *Service) TerminalStatus() map[string]any {
	if !s.TerminalAccessEnabled() {
		pin := s.isPinEnabled()
		return map[string]any{"enabled": false, "ready": false, "shortcutReady": false, "state": "disabled", "label": "Disabled by SSH administrator", "pinEnabled": pin, "authorization": ternary(pin, "Control session", "No PIN required"), "hint": "Terminal access is disabled by the SSH administrator.", "problems": []any{}}
	}
	xterm, _ := exec.LookPath("xterm")
	xbind, _ := exec.LookPath("xbindkeys")
	display := fileio.Exists("/tmp/.X11-unix/X0")
	script := filepath.Join(s.binDir, "dashboard-terminal.sh")
	exists := fileio.Exists(script)
	executable := false
	if st, e := os.Stat(script); e == nil {
		executable = st.Mode()&0111 != 0
	}
	shortcutConf := filepath.Join(s.cacheDir, "dashboard-xbindkeys.conf")
	configured := fileio.Exists(shortcutConf)
	running := false
	if out := CommandText(5*time.Second, "pgrep", "-af", "xbindkeys"); strings.TrimSpace(out) != "" && !strings.HasPrefix(out, "unavailable:") {
		running = strings.Contains(out, "dashboard-xbindkeys.conf") || strings.Contains(out, "xbindkeys")
	}
	browser := false
	if out := CommandText(5*time.Second, "pgrep", "-x", "surf"); strings.TrimSpace(out) != "" && !strings.HasPrefix(out, "unavailable:") {
		browser = true
	}
	problems := []any{}
	if !exists {
		problems = append(problems, "terminal wrapper is missing")
	} else if !executable {
		problems = append(problems, "terminal wrapper is not executable")
	}
	if xterm == "" {
		problems = append(problems, "xterm is not installed")
	}
	if !display {
		problems = append(problems, "X display :0 is not available")
	}
	if xbind == "" {
		problems = append(problems, "xbindkeys is not installed; browser fallback may still work")
	} else if !running {
		problems = append(problems, "global Ctrl+Alt+T shortcut is not running yet")
	}
	ready := exists && executable && xterm != "" && display
	shortcut := xbind != "" && configured && running
	state, label := "action", "Setup needed"
	if ready && shortcut {
		state, label = "healthy", "Ready"
	} else if ready {
		state, label = "check", "Button ready"
	}
	pin := s.isPinEnabled()
	return map[string]any{"enabled": true, "ready": ready, "shortcutReady": shortcut, "state": state, "label": label, "script": script, "scriptExists": exists, "scriptExecutable": executable, "xterm": xterm != "", "xtermPath": xterm, "xbindkeys": xbind != "", "xbindkeysPath": xbind, "shortcutConfigured": configured, "shortcutRunning": running, "displayAvailable": display, "browserRunning": browser, "pinEnabled": pin, "authorization": ternary(pin, "Control session", "No PIN required"), "hint": "Type exit or close the terminal to return to the fullscreen dashboard.", "repairHint": "sudo apt-get install -y xterm xbindkeys", "problems": problems}
}
func (s *Service) OpenTerminal() (map[string]any, error) {
	if !s.TerminalAccessEnabled() {
		return nil, ErrTerminalAccessDisabled
	}
	st := s.TerminalStatus()
	if st["scriptExists"] != true {
		return nil, fmt.Errorf("terminal wrapper is missing: %s", st["script"])
	}
	if st["xterm"] != true {
		return nil, fmt.Errorf("xterm is not installed. Fix: %s", st["repairHint"])
	}
	if st["displayAvailable"] != true {
		return nil, errors.New("X display :0 is not available")
	}
	_ = os.MkdirAll(s.logDir, 0755)
	logPath := filepath.Join(s.logDir, "terminal.log")
	f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if f != nil {
		_, _ = fmt.Fprintf(f, "[%s] opening terminal from Dashboard Control\n", s.nowTime().Format("2006-01-02 15:04:05"))
	}
	cmd := exec.Command("bash", filepath.Join(s.binDir, "dashboard-terminal.sh"), "--authorized")
	cmd.Dir = s.dashDir
	cmd.Env = append(os.Environ(), "DISPLAY=:0", "XAUTHORITY="+filepath.Join(s.homeDir, ".Xauthority"))
	if f != nil {
		cmd.Stdout = f
		cmd.Stderr = f
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e := cmd.Start(); e != nil {
		if f != nil {
			_ = f.Close()
		}
		return nil, e
	}
	if f != nil {
		_ = f.Close()
	}
	return map[string]any{"opened": true, "status": st}, nil
}

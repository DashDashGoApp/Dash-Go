package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func (a *app) runPinCheckCLI(args []string) int {
	fs := flag.NewFlagSet("pin-check", flag.ContinueOnError)
	pin := fs.String("pin", os.Getenv("PIN_VALUE"), "PIN to verify")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	cfg := a.authService().Config()
	if !cfg.Available {
		fmt.Fprintln(os.Stderr, "Dashboard Control PIN configuration is unavailable")
		return 1
	}
	if !cfg.Enabled {
		return 0
	}
	if a.verifyPin(strings.TrimSpace(*pin)) {
		return 0
	}
	return 1
}

func (a *app) runJSONValidateCLI(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: --json-validate FILE")
		return 64
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (a *app) runJSONGetCLI(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: --json-get FILE FIELD [DEFAULT]")
		return 64
	}
	dflt := ""
	if len(args) > 2 {
		dflt = args[2]
	}
	raw := a.readJSONDefault(args[0], map[string]any{})
	cur := any(raw)
	for part := range strings.SplitSeq(args[1], ".") {
		if part == "" {
			continue
		}
		m := jsonutil.Map(cur)
		if v, ok := m[part]; ok {
			cur = v
		} else {
			fmt.Print(dflt)
			return 0
		}
	}
	switch v := cur.(type) {
	case string:
		fmt.Print(v)
	case float64:
		if v == float64(int64(v)) {
			fmt.Print(int64(v))
		} else {
			fmt.Print(v)
		}
	case bool:
		if v {
			fmt.Print("true")
		} else {
			fmt.Print("false")
		}
	default:
		b, _ := json.Marshal(v)
		fmt.Print(string(b))
	}
	return 0
}

func (a *app) runWriteStatusCLI(args []string) int {
	fs := flag.NewFlagSet("write-status", flag.ContinueOnError)
	file := fs.String("file", "", "status JSON file")
	state := fs.String("state", "", "state")
	detail := fs.String("detail", "", "detail")
	label := fs.String("label", "", "label")
	rc := fs.String("rc", "", "return code")
	commandPID := fs.String("command-pid", "", "system update command process ID")
	kind := fs.String("kind", "generic", "status kind")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file required")
		return 64
	}
	now := time.Now().Unix()
	m := jsonutil.Map(a.readJSONDefault(*file, map[string]any{}))
	if *state != "" {
		m["state"] = *state
	}
	if *detail != "" {
		m["detail"] = *detail
	}
	if *label != "" {
		m["label"] = *label
	}
	if *rc != "" {
		if n, err := strconv.Atoi(*rc); err == nil {
			m["returnCode"] = n
			m["rc"] = n
		} else {
			m["rc"] = *rc
		}
	}
	if *commandPID != "" {
		n, err := strconv.Atoi(*commandPID)
		if err != nil || n <= 0 {
			fmt.Fprintln(os.Stderr, "--command-pid must be a positive integer")
			return 64
		}
		m["commandPid"] = n
	}
	m["updatedAt"] = now
	if *kind == "session-guard" {
		m["updated"] = now
		m["display"] = os.Getenv("DISPLAY")
	}
	if *kind == "maintenance" {
		m["task"] = *label
	}
	if err := fileio.WriteJSON(*file, m); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

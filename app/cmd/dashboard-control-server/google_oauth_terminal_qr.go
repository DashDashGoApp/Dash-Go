package main

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

func googleOAuthMaybeQR(authorizationURL string, stderr io.Writer) {
	qrencode, err := exec.LookPath("qrencode")
	if err != nil {
		return
	}
	encoded, err := exec.Command(qrencode, "-t", "ANSIUTF8", authorizationURL).Output()
	if err != nil || len(encoded) == 0 {
		return
	}
	fmt.Fprintf(stderr, "\nOr scan this with your phone:\n\n%s\n", encoded)
	if rows := googleOAuthTerminalRows(); rows > 0 && strings.Count(string(encoded), "\n") > rows {
		fmt.Fprintln(stderr, "The terminal QR is taller than this window; enlarge it or use the printed link instead.")
	}
}

func googleOAuthTerminalRows() int {
	output, err := exec.Command("stty", "size").Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(output))
	if len(fields) != 2 {
		return 0
	}
	rows, err := strconv.Atoi(fields[0])
	if err != nil || rows <= 0 {
		return 0
	}
	return rows
}

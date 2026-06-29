package main

import (
	"flag"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Updater capability discovery is deliberately a tiny, explicit Go CLI. The
// installer relies on this only after a versioned bridge has installed a binary
// that knows the command; it never reads shipped Go source to infer behavior.
const updaterCapabilityProtocol = "dash-go-updater-capabilities-v1"

func updaterCapabilities() []string {
	return []string{
		updaterCapabilityProtocol,
		"release-file-list-v1",
		"release-manifest-v1",
		"github-release-resolution-v3",
		"stale-source-purge-v1",
		"update-job-v1",
		"update-status-v1",
		"update-action-history-v1",
	}
}

func updaterHasCapability(name string) bool {
	for _, capability := range updaterCapabilities() {
		if capability == name {
			return true
		}
	}
	return false
}

func (a *app) runUpdaterCapabilitiesCLI(args []string) int {
	fs := flag.NewFlagSet("updater-capabilities", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(fs.Output(), "usage: --updater-capabilities")
		return 64
	}
	for _, capability := range updaterCapabilities() {
		fmt.Println(capability)
	}
	return 0
}

// runWriteUpdaterMigrationCLI writes diagnostic evidence after a bridge update
// verifies the newly installed Go updater. The receipt is not a security
// decision: later updates ask the live binary for capabilities again.
func (a *app) runWriteUpdaterMigrationCLI(args []string) int {
	fs := flag.NewFlagSet("write-updater-migration", flag.ContinueOnError)
	file := fs.String("file", "", "private receipt path")
	previous := fs.String("previous-version", "", "prior installed version")
	arch := fs.String("architecture", "", "selected host architecture")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		fmt.Fprintln(fs.Output(), "usage: --write-updater-migration --file FILE [--previous-version VERSION] [--architecture ARCH]")
		return 64
	}
	if strings.TrimSpace(*file) == "" {
		fmt.Fprintln(fs.Output(), "--file required")
		return 64
	}
	capabilities := append([]string(nil), updaterCapabilities()...)
	slices.Sort(capabilities)
	receipt := map[string]any{
		"schema":               1,
		"installedVersion":     a.releaseVersion,
		"previousVersion":      strings.TrimSpace(*previous),
		"architecture":         strings.TrimSpace(*arch),
		"capabilities":         capabilities,
		"capabilityQuery":      updaterCapabilityProtocol,
		"verifiedAt":           time.Now().Unix(),
		"releaseManifestReady": updaterHasCapability("release-manifest-v1"),
	}
	if err := writeJSONPrivateFile(*file, receipt); err != nil {
		fmt.Fprintln(fs.Output(), err)
		return 1
	}
	return 0
}

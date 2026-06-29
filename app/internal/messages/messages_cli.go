package messages

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// runMessageSourcesCLI exposes the same catalog and validated preference writer
// used by Dashboard Control. The installer uses it instead of maintaining a
// second, stale source list in shell or Python.
func (s *Service) runMessageSourcesCLI(args []string) int {
	fs := flag.NewFlagSet("message-sources", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	list := fs.Bool("list", false, "list category id, label, and adult flag as TSV")
	set := fs.String("set", "", "comma-separated category IDs; empty clears all")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	hasSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "set" {
			hasSet = true
		}
	})
	if *list == hasSet {
		fmt.Fprintln(os.Stderr, "use exactly one of --list or --set")
		return 64
	}
	if *list {
		enabled := map[string]bool{}
		for _, raw := range s.normalizeMessageEnabled(jsonutil.List(s.messagePrefs()["enabled"])) {
			enabled[fmt.Sprint(raw)] = true
		}
		for _, c := range messageCategories {
			fmt.Printf("%s\t%s\t%t\t%t\n", c.ID, c.Label, c.NSFW, enabled[c.ID])
		}
		return 0
	}
	values := []any{}
	valid := map[string]bool{}
	for _, category := range messageCategories {
		valid[category.ID] = true
	}
	for raw := range strings.SplitSeq(*set, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !valid[id] {
			fmt.Fprintln(os.Stderr, "one or more message category IDs are unknown")
			return 64
		}
		values = append(values, id)
	}
	enabled := s.normalizeMessageEnabled(values)
	prefs := map[string]any{"enabled": enabled, "updatedAt": nowMillis()}
	if err := fileio.WriteJSON(filepath.Join(s.configDir, "message-sources.json"), prefs); err != nil {
		fmt.Fprintln(os.Stderr, "write message-source preferences:", err)
		return 1
	}
	fmt.Printf("enabled %d message source(s)\n", len(enabled))
	return 0
}

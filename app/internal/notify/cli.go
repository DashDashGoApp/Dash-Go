package notify

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	apprise "github.com/unraid/apprise-go"
)

func cliUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: dashboard-control-server --apprise-status|--apprise-people|--apprise-route-set --person ID|--apprise-route-remove --person ID|--apprise-test --person ID|--apprise-set-enabled true|false|--apprise-remove-orphaned-routes|--apprise-remove-config")
	return 64
}

func cliFlag(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return strings.TrimSpace(args[i+1])
		}
	}
	return ""
}

// RunCLI retains the current installer-facing commands and output while keeping
// route parsing, private-store changes, and direct test delivery inside the
// Notifications bounded context.
func (s *Service) RunCLI(command string, args []string) int {
	return s.RunCLIWithIO(command, args, os.Stdin, os.Stdout, os.Stderr)
}

func (s *Service) RunCLIWithIO(command string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch command {
	case "--apprise-status":
		routes := s.Routes()
		fmt.Fprintf(stdout, "Apprise-Go external delivery: %s\n", map[bool]string{true: "enabled", false: "disabled"}[routes.Enabled])
		for _, person := range s.ConfiguredPeople() {
			state := "not configured"
			if person.Configured {
				state = "configured"
			}
			fmt.Fprintf(stdout, "- %s: %s\n", person.Name, state)
		}
		if orphaned := len(s.OrphanRouteIDs()); orphaned > 0 {
			fmt.Fprintf(stdout, "- Removed-People routes: %d preserved; remove them through Notifications if no longer needed.\n", orphaned)
		}
		return 0
	case "--apprise-people":
		for _, person := range s.ConfiguredPeople() {
			if person.State == "active" {
				fmt.Fprintf(stdout, "%s\t%s\t%t\n", person.ID, person.Name, person.Configured)
			}
		}
		return 0
	case "--apprise-route-set":
		personID := s.normalizePersonID(cliFlag(args, "--person"))
		if !s.activePerson(personID) {
			fmt.Fprintln(stderr, "active household person is required")
			return 64
		}
		rows := []string{}
		scanner := bufio.NewScanner(stdin)
		scanner.Buffer(make([]byte, 1024), MaxRouteLength+1024)
		for scanner.Scan() {
			route := strings.TrimSpace(scanner.Text())
			if route == "" {
				continue
			}
			if len(route) > MaxRouteLength || len(rows) >= MaxRoutesPerPerson {
				fmt.Fprintln(stderr, "invalid number or length of Apprise routes")
				return 64
			}
			client := apprise.New()
			if err := client.Add(route); err != nil {
				fmt.Fprintln(stderr, "one or more Apprise routes could not be parsed")
				return 64
			}
			rows = append(rows, route)
		}
		if err := scanner.Err(); err != nil || len(rows) == 0 {
			fmt.Fprintln(stderr, "at least one valid Apprise route is required")
			return 64
		}
		store := s.Routes()
		store.Routes[personID] = rows
		if err := s.SaveRoutes(store); err != nil {
			fmt.Fprintln(stderr, "could not save private Apprise configuration")
			return 1
		}
		fmt.Fprintln(stdout, "Apprise route saved. Route values are intentionally not displayed.")
		return 0
	case "--apprise-route-remove":
		personID := s.normalizePersonID(cliFlag(args, "--person"))
		store := s.Routes()
		delete(store.Routes, personID)
		if err := s.SaveRoutes(store); err != nil {
			fmt.Fprintln(stderr, "could not remove private Apprise route")
			return 1
		}
		fmt.Fprintln(stdout, "Apprise route removed.")
		return 0
	case "--apprise-test":
		personID := s.normalizePersonID(cliFlag(args, "--person"))
		if !s.activePerson(personID) || !s.ConfiguredForPerson(personID) {
			fmt.Fprintln(stderr, "an active person with a configured Apprise route is required")
			return 64
		}
		s.Deliver(Event{PersonID: personID, Title: "Dash-Go Apprise-Go test", Body: "Dash-Go external delivery is configured.", Warning: false})
		status := s.PersonPreferences(personID).LastState
		if status != "delivered" {
			fmt.Fprintln(stderr, "Apprise test delivery did not complete")
			return 1
		}
		fmt.Fprintln(stdout, "Apprise test delivery completed.")
		return 0
	case "--apprise-set-enabled":
		if len(args) != 1 || (args[0] != "true" && args[0] != "false") {
			return cliUsage(stderr)
		}
		store := s.Routes()
		store.Enabled = args[0] == "true"
		if err := s.SaveRoutes(store); err != nil {
			fmt.Fprintln(stderr, "could not update Apprise delivery state")
			return 1
		}
		fmt.Fprintf(stdout, "Apprise external delivery %s.\n", map[bool]string{true: "enabled", false: "disabled"}[store.Enabled])
		return 0
	case "--apprise-remove-orphaned-routes":
		removed, err := s.RemoveOrphanRoutes()
		if err != nil {
			fmt.Fprintln(stderr, "could not remove orphaned private Apprise routes")
			return 1
		}
		fmt.Fprintf(stdout, "Removed %d private route(s) for permanently removed People.\n", removed)
		return 0
	case "--apprise-remove-config":
		if err := s.RemoveRoutesConfig(); err != nil {
			fmt.Fprintln(stderr, "could not remove private Apprise configuration")
			return 1
		}
		fmt.Fprintln(stdout, "Apprise routes removed. Local People preferences were retained.")
		return 0
	default:
		return cliUsage(stderr)
	}
}

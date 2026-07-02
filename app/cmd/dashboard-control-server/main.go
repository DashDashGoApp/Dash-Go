package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	releasepkg "github.com/DashDashGoApp/Dash-Go/app/internal/release"

	controlauth "github.com/DashDashGoApp/Dash-Go/app/internal/auth"
	calendarpkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar"
	eventspkg "github.com/DashDashGoApp/Dash-Go/app/internal/calendar/events"
	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	householdpkg "github.com/DashDashGoApp/Dash-Go/app/internal/household"
	chorepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/chores"
	familypkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/family"
	maintenancepkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/maintenance"
	routinespkg "github.com/DashDashGoApp/Dash-Go/app/internal/household/routines"
	mapspkg "github.com/DashDashGoApp/Dash-Go/app/internal/maps"
	messagespkg "github.com/DashDashGoApp/Dash-Go/app/internal/messages"
	notifypkg "github.com/DashDashGoApp/Dash-Go/app/internal/notify"
	platformpkg "github.com/DashDashGoApp/Dash-Go/app/internal/platform"
	settingspkg "github.com/DashDashGoApp/Dash-Go/app/internal/settings"
	todopkg "github.com/DashDashGoApp/Dash-Go/app/internal/todo"
	weatherpkg "github.com/DashDashGoApp/Dash-Go/app/internal/weather"
)

// app holds process-wide runtime paths and the small mutable state that spans HTTP requests.
// Domain behavior lives in focused companion files.
type app struct {
	dash                    string
	home                    string
	configDir               string
	calDir                  string
	cacheDir                string
	logDir                  string
	binDir                  string
	settingsFile            string
	configLocal             string
	celebrationsFile        string
	todoDir                 string
	todoTokenFile           string
	fontsDir                string
	authInitMu              sync.Mutex
	auth                    *controlauth.Service
	settingsInitMu          sync.Mutex
	settings                *settingspkg.Service
	weatherInitMu           sync.Mutex
	weather                 *weatherpkg.Service
	eventsInitMu            sync.Mutex
	events                  *eventspkg.Service
	calendarInitMu          sync.Mutex
	calendar                *calendarpkg.Service
	mapsInitMu              sync.Mutex
	maps                    *mapspkg.Service
	messagesInitMu          sync.Mutex
	messages                *messagespkg.Service
	notifyInitMu            sync.Mutex
	notify                  *notifypkg.Service
	householdInitMu         sync.Mutex
	household               *householdpkg.Service
	familyInitMu            sync.Mutex
	family                  *familypkg.Service
	choresInitMu            sync.Mutex
	chores                  *chorepkg.Service
	maintenanceInitMu       sync.Mutex
	maintenance             *maintenancepkg.Service
	routinesInitMu          sync.Mutex
	routines                *routinespkg.Service
	updateMu                sync.Mutex
	updateAvailabilityMu    sync.Mutex
	updateAvailabilityCache map[string]any
	updateAvailabilityAt    time.Time
	platformInitMu          sync.Mutex
	platform                *platformpkg.Service
	// The To Do service owns mutable sync, queue, auth, cache, migration, and
	// Grocery Memory state. Core retains immutable paths plus the lazy service
	// reference and HTTP/SSE adapter state only.
	todoInitMu     sync.Mutex
	todo           *todopkg.Service
	todoStreamMu   sync.Mutex
	todoStreams    map[chan []byte]bool
	releaseVersion string
	// releaseResolver is an in-process test seam; production always uses the
	// canonical GitHub Release client.
	releaseResolver func(context.Context, releasepkg.Track) (releasepkg.Resolved, error)
}

func main() {
	a := newAppFromRuntime()
	a.ensureDirs()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--gen-events-cache":
			os.Exit(a.runEventCacheCLI(os.Args[2:]))
		case "--gen-calendars":
			os.Exit(a.runCalendarManifestCLI(os.Args[2:]))
		case "--gen-default-calendars":
			os.Exit(a.runDefaultCalendarsCLI(os.Args[2:]))
		case "--update-holidays":
			os.Exit(a.runHolidayUpdateCLI(os.Args[2:]))
		case "--update-iss-passes":
			os.Exit(a.runISSPassesCLI(os.Args[2:]))
		case "--seasonal-theme":
			os.Exit(a.runSeasonalThemeCLI(os.Args[2:]))
		case "--maps-prewarm":
			os.Exit(a.runMapPrewarmCLI(os.Args[2:]))
		case "--update-message-feeds":
			os.Exit(a.runUpdateMessageFeedsCLI(os.Args[2:]))
		case "--message-sources":
			os.Exit(a.runMessageSourcesCLI(os.Args[2:]))
		case "--migrate-compliments":
			os.Exit(a.runMigrateComplimentsCLI(os.Args[2:]))
		case "--setup-demo-mode":
			os.Exit(a.runSetupDemoModeCLI(os.Args[2:]))
		case "--doctor-config":
			os.Exit(a.runDoctorConfigCLI(os.Args[2:]))
		case "--doctor-data":
			os.Exit(a.runDoctorDataCLI(os.Args[2:]))
		case "--settings-validate":
			os.Exit(a.runSettingsValidateCLI(os.Args[2:]))
		case "--refresh-weather":
			os.Exit(a.runRefreshWeatherCLI(os.Args[2:]))
		case "--pin-check":
			os.Exit(a.runPinCheckCLI(os.Args[2:]))
		case "--terminal-access":
			os.Exit(a.runTerminalAccessCLI(os.Args[2:]))
		case "--json-validate":
			os.Exit(a.runJSONValidateCLI(os.Args[2:]))
		case "--json-get":
			os.Exit(a.runJSONGetCLI(os.Args[2:]))
		case "--write-status":
			os.Exit(a.runWriteStatusCLI(os.Args[2:]))
		case "--update-status":
			os.Exit(a.runUpdateStatusCLI(os.Args[2:]))
		case "--update-job":
			os.Exit(a.runUpdateJobCLI(os.Args[2:]))
		case "--finalize-update-action":
			os.Exit(a.runFinalizeUpdateActionCLI(os.Args[2:]))
		case "--verify-release-manifest":
			os.Exit(a.runVerifyReleaseManifestCLI(os.Args[2:]))
		case "--resolve-github-release":
			os.Exit(a.runResolveGitHubReleaseCLI(os.Args[2:]))
		case "--release-file-list":
			os.Exit(a.runReleaseFileListCLI(os.Args[2:]))
		case "--purge-stale-managed":
			os.Exit(a.runPurgeStaleManagedCLI(os.Args[2:]))
		case "--updater-capabilities":
			os.Exit(a.runUpdaterCapabilitiesCLI(os.Args[2:]))
		case "--write-updater-migration":
			os.Exit(a.runWriteUpdaterMigrationCLI(os.Args[2:]))
		case "--apprise-status", "--apprise-people", "--apprise-route-set", "--apprise-route-remove", "--apprise-test", "--apprise-set-enabled", "--apprise-remove-orphaned-routes", "--apprise-remove-config":
			os.Exit(a.runAppriseCLI(os.Args[1], os.Args[2:]))
		case "--verify-generated-assets":
			os.Exit(a.runVerifyGeneratedAssetsCLI(os.Args[2:]))
		case "--verify-package-scrub":
			os.Exit(a.runPackageScrubCLI(os.Args[2:]))
		case "--compliments":
			os.Exit(a.runComplimentsCLI(os.Args[2:]))
		case "--apply-profile-preset":
			os.Exit(a.runApplyProfilePresetCLI(os.Args[2:]))
		default:
			if len(os.Args[1]) > 2 && os.Args[1][:2] == "--" {
				fmt.Fprintf(os.Stderr, "unknown dashboard-control-server command: %s\n", os.Args[1])
				os.Exit(64)
			}
		}
	}
	// A server restart during an update can happen between writing the terminal
	// job record and the runner finalizing Recent Actions. Repair that narrow
	// window before the HTTP server accepts its first request.
	a.reconcileInterruptedUpdateState()
	a.reconcileUpdateActionHistory()
	a.ensureSettingsSafeAtBoot()
	if err := a.todoNormalizeInboundSyncSetting(); err != nil {
		log.Printf("could not normalize retired Microsoft To Do cadence: %v", err)
	}
	a.startTodoArchiveJanitor()
	a.startTodoInboundScheduler()
	a.startAppriseNotifier()
	// settings.json is user-owned state. Do not create or reseed it at startup:
	// config.local.js supplies fresh-install defaults and normal updates must not
	// replace personal settings with profile defaults.
	port := envInt("DASH_CONTROL_PORT", 8090, 1024, 65535)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("dashboard Go control server on http://%s (dir %s)", addr, a.dash)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serveHTTPUntilSignal(ctx, a.httpServer(addr), listener, 8*time.Second); err != nil {
		log.Fatal(err)
	}
}

func newAppFromRuntime() *app {
	exe, _ := os.Executable()
	dash := filepath.Dir(filepath.Dir(exe))
	if _, err := os.Stat(filepath.Join(dash, "index.html")); err != nil {
		if wd, err := os.Getwd(); err == nil {
			dash = wd
		}
	}
	home, _ := os.UserHomeDir()
	a := &app{dash: dash, home: home, configDir: filepath.Join(dash, "config"), calDir: filepath.Join(dash, "calendars"), cacheDir: filepath.Join(dash, "cache"), logDir: filepath.Join(dash, "logs"), binDir: filepath.Join(dash, "bin"), settingsFile: filepath.Join(dash, "config", "settings.json"), configLocal: filepath.Join(dash, "config", "config.local.js"), celebrationsFile: filepath.Join(home, ".dashboard-celebrations"), todoDir: filepath.Join(dash, "config", "todo"), todoTokenFile: filepath.Join(home, ".dashboard-todo.json"), fontsDir: filepath.Join(dash, "fonts"), todoStreams: map[chan []byte]bool{}, releaseVersion: fileio.ReadString(filepath.Join(dash, "VERSION"), "")}
	a.settings = settingspkg.New(a.settingsConfig())
	return a
}
func envInt(k string, def, lo, hi int) int {
	n, err := strconv.Atoi(os.Getenv(k))
	if err != nil || n < lo || n > hi {
		return def
	}
	return n
}
func (a *app) ensureDirs() {
	for _, d := range []string{a.configDir, a.calDir, a.cacheDir, a.logDir, a.todoDir, a.fontsDir} {
		_ = os.MkdirAll(d, 0755)
	}
	// Calendar Trash is bounded user-recovery data. Purge it once per server
	// start; Calendar Manager also checks it before rendering its restore list.
	_ = a.purgeExpiredCalendarTrash()
}

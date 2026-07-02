#!/usr/bin/env bash
# Release-blocking static contract smoke for retained dashboard contracts and 1.4.3-beta.10 startup integrity, interaction, Lite-radar observed-history bounds, and static staged day-timeline coverage.
# It deliberately avoids network, X11, and systemd.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Source handoffs keep the installer as a sibling directory. Extracted payload
# runs receive it explicitly from the package verifier because the package verifier supplies it
# at the document-root level, outside the application tarball.
INSTALLER="${DASHGO_INSTALLER_UNDER_TEST:-$ROOT/../installer/install.sh}"
[ -f "$INSTALLER" ] || { echo "FAIL: installer under test not found: $INSTALLER" >&2; exit 1; }
need(){ grep -Fq -- "$2" "$1" || { echo "FAIL: missing $3" >&2; exit 1; }; }
absent(){ ! grep -Fq -- "$2" "$1" || { echo "FAIL: forbidden $3" >&2; exit 1; }; }

TAP="$ROOT/ui/js/tap.js"
MSG="$ROOT/ui/js/messages-options.js"
BOOT="$ROOT/ui/js/boot.js"
APP_LAUNCHER="$ROOT/ui/js/app-launcher.js"
RADAR="$ROOT/ui/js/radar-overlay.js"
LITE_RADAR="$ROOT/ui/js/radar-lite.js"
WEATHER="$ROOT/ui/js/weather.js"
SOURCES="$ROOT/ui/js/radar-sources.js"
CONFIG_DEFAULTS="$ROOT/ui/js/config-defaults.js"
SETTINGS="$ROOT/ui/js/settings-runtime.js"
THEME_RUNTIME="$ROOT/ui/js/config-runtime.js"
SEASONAL_DECOR="$ROOT/ui/js/calendar-seasonal-decor.js"
HEALTH="$ROOT/ui/js/display-health.js"
MSG_OVERLAY="$ROOT/ui/js/popup-overlays.js"
GOJSON="$ROOT/internal/jsonutil/jsonutil.go"
GOHEALTH="$ROOT/internal/platform/health.go"
DEMO="$ROOT/cmd/dashboard-control-server/demo_cli.go"
CSS="$ROOT/ui/css/dashboard/sidebar-weather-messages.css"
TOUCH_CSS="$ROOT/ui/css/dashboard/touch-radar.css"
TIMELINE_CSS="$ROOT/ui/css/dashboard/day-timeline-popup.css"
DAY_POPUP="$ROOT/ui/js/day-popup.js"
GUARD="$ROOT/bin/dashboard-health-guard.sh"
CTRL_CSS="$ROOT/ui/css/control/consistency.css"
SERVER="$ROOT/cmd/dashboard-control-server"
WEATHER_DOMAIN="$ROOT/internal/weather"
SETTINGS_DOMAIN="$ROOT/internal/settings"
MESSAGES_DOMAIN="$ROOT/internal/messages"
RECUR="$ROOT/internal/calendar/events/recurrence.go"
ICS="$ROOT/internal/calendar/events/ics.go"
EVENT_UTILS="$ROOT/internal/calendar/events/helpers.go"
WEATHER_HTTP="$WEATHER_DOMAIN/weather_keyed_http.go"
OPENMETEO="$WEATHER_DOMAIN/weather_openmeteo.go"
ISS="$ROOT/internal/calendar/iss.go"
CALENDAR_SERVICE="$ROOT/internal/calendar/service.go"
SETTINGS_GO="$SERVER/settings.go"
SETTINGS_SERVICE="$SETTINGS_DOMAIN/service.go"
BLEND="$WEATHER_DOMAIN/weather_blend_go.go"

need "$TAP" 'function attachTaps' 'unified attachTaps core'
need "$TAP" 'window.PointerEvent' 'Pointer Events path'
need "$TAP" 'suppressClickUntil' 'synthetic click suppression'
need "$TAP" 'function bindTripleTap' 'public triple-tap binder'
need "$TAP" 'function bindSingleDoubleTap' 'public single/double binder'
absent "$TAP" 'lastFire' 'old global re-fire lock'
need "$MSG" 'attachTaps(target' 'message gesture uses unified tap core'
need "$MSG" 'acquireMessageRotationPause("message-tap",2500)' 'first-tap message rotation freeze'
need "$MSG" 'gap:800,moveTol:32' 'generous message gesture window'
need "$MSG" 'requestAnimationFrame' 'scrim-first popup reveal'
need "$MSG" 'ensureMessagePopupShell' 'reusable message popup shell'
absent "$MSG" 'MutationObserver' 'per-open message popup observer'
need "$MSG_OVERLAY" 'releaseMessagePopupRotationPause' 'single close-path popup pause release'
need "$SETTINGS" 'CONFIG.lat=+s.lat' 'runtime latitude propagation'
need "$SETTINGS" 'CONFIG.lon=+s.lon' 'runtime longitude propagation'
need "$HEALTH" 'sline===' 'browser stale-pill sentinel filtering'
need "$GOJSON" 'if v == nil' 'Go null-safe shared conversion'
need "$GOHEALTH" 'case "recovered", "running"' 'running guard health mapping'
need "$GOHEALTH" 'fact.Name + " is " + fact.Level' 'health fact fallback construction'
need "$BOOT" 'bindTripleTap(moonButton' 'moon triple-tap binding'
need "$BOOT" 'bindTripleTap(wxPanel' 'weather-panel triple-tap binding'
need "$BOOT" 'pulseTapAffordance' 'first-tap affordance'
need "$BOOT" 'bindTap(cbButton,openAppLauncher)' 'footer app-launcher binding'
need "$APP_LAUNCHER" 'const DASHBOARD_APPS' 'static app registry'
need "$APP_LAUNCHER" 'function openChalkboard' 'retained Chalkboard entry point'
need "$APP_LAUNCHER" 'function updateAppLauncherTrigger' 'app availability refresh'
need "$CSS" 'touch-action:manipulation' 'message-bar manipulation touch action'
absent "$CSS" 'touch-action:none' 'message-bar touch-action none'
need "$TOUCH_CSS" 'min-width:96px' 'moon minimum touch width'
need "$TOUCH_CSS" 'min-height:96px' 'moon minimum touch height'
need "$CTRL_CSS" 'min-height:48px' 'control 48px targets'
need "$SOURCES" 'RADAR_SOURCE_META' 'radar provider registry'
need "$SOURCES" 'rainviewer' 'RainViewer provider'
need "$SOURCES" 'weatherbit' 'Weatherbit provider'
need "$SOURCES" 'xweather' 'Xweather provider'
need "$RADAR" 'RADAR_IDLE_MS' 'radar idle auto-close'
need "$SOURCES" 'function radarFrameCount' 'source-owned frame count'
need "$SOURCES" 'frameMode:"source"' 'RainViewer source timeline metadata'
absent "$SOURCES" 'function radarFrameBudget' 'retired profile frame budget'
absent "$SOURCES" 'function radarHistoryModeForTier' 'retired observed-history tuning'
need "$RADAR" 'radarPool' 'bounded request queue'
need "$RADAR" 'tileCache' 'bounded tile cache'
need "$RADAR" 'function radarEnsureGrid' 'build-once grid helper'
need "$RADAR" 'dataset.gridSig' 'grid signature cache'
need "$RADAR" 'radarCenterFraction' 'configured-coordinate centering'
need "$RADAR" 'radarSkipBase' 'Lite radar-only base skip'
need "$RADAR" 'radarPrefetchConcurrency' 'profile-scaled concurrency'
need "$RADAR" 'radarSchedulePrefetch' 'lazy bounded prefetch'
need "$RADAR" 'radar-frame-layer' 'decoded-frame crossfade layers'
absent "$RADAR" 'function radarBuildGrid' 'per-frame rebuilding grid helper'
need "$RADAR" '/api/radar/tile' 'keyed server proxy route'
need "$RADAR" "radarUseFallback" "ordered visible provider fallback"
need "$RADAR" "function radarPaintGrid" "square measured-pixel tile painter"
need "$RADAR" "tilePx=Math.ceil(Math.max(W,H)/(RADAR_TILE_GRID-1))" "square tile geometry"
need "$RADAR" "img.style.width=img.style.height=tilePx" "square pixel tile sizing"
need "$RADAR" "function radarJumpLatest" "latest-frame control"
need "$RADAR" "function radarCanAnimate" "manual-only Lite animation guard"
need "$RADAR" "radarSetPlaying(false);radarSchedulePrefetch(token);" "paused latest frame on open/fallback"
need "$RADAR" "RADAR_STATE.fullFrameFailures<2" "two-full-frame fallback threshold"
absent "$RADAR" "RADAR_TILE_COVERAGE" "old percentage tile stretching math"
need "$RADAR" "RADAR_PREFETCH_NEAREST_FRAMES=2" "bounded nearest-frame prefetch"
need "$RADAR" "return past;" "complete RainViewer source timeline"
absent "$RADAR" "past.slice(" "profile-capped RainViewer frame timeline"
need "$RADAR" "RADAR_LITE_RENDER_PX=640" "fixed Lite canvas budget"
need "$RADAR" "RADAR_LITE_FRAME_MS=650" "Lite playback cadence"
need "$RADAR" "RADAR_LITE_REFRESH_MS=20000" "Lite refresh cooldown"
need "$LITE_RADAR" "function radarLiteFrameLimitForZoom" "source-driven Lite frame selection"
absent "$LITE_RADAR" "radarHistoryModeForTier" "retired Lite history setting"
absent "$CONFIG_DEFAULTS" 'radarHistoryMode: "auto"' "retired observed-history configuration"
absent "$CONFIG_DEFAULTS" "radarLiteFrames: 3" "retired Lite snapshot setting"
absent "$CONFIG_DEFAULTS" "radarLiteMaxPx: 640" "retired Lite canvas setting"
absent "$SETTINGS" 'radarHistoryMode:"auto"' "retired runtime observed-history default"
absent "$SETTINGS" "CONFIG.radarHistoryMode" "retired runtime observed-history hydration"
need "$LITE_RADAR" "function radarLitePx" "stage-sized Lite backing surface"
need "$LITE_RADAR" "function radarBuildLiteBase" "shared Lite base compositor"
need "$LITE_RADAR" "function radarBuildLiteFrames" "bounded Lite frame compositor"
need "$LITE_RADAR" "function radarDrawLiteFrame" "Lite bitmap blitter"
need "$LITE_RADAR" "function radarStopLiteAnim" "Lite timer cleanup"
need "$LITE_RADAR" "function radarFreeLiteFrames" "Lite bitmap/base cleanup"
need "$LITE_RADAR" "function radarLiteToggleZoom" "Lite zoom rebuild"
need "$LITE_RADAR" "function radarLiteBaseConcurrency" "Lite OSM concurrency cap"
need "$LITE_RADAR" "function radarLiteRadarConcurrency" "Lite RainViewer concurrency cap"
need "$LITE_RADAR" "function radarLiteBaseFor" "per-zoom Lite base cache"
need "$LITE_RADAR" "function radarFreeLiteBaseCache" "Lite base-cache cleanup"
need "$LITE_RADAR" "function radarCommitLiteView" "newest-frame atomic Lite commit"
need "$LITE_RADAR" "radarSetBusy(false)" "first-frame spinner release"
need "$LITE_RADAR" "liteFramesFetchedAt" "zoom frame-list reuse timestamp"
need "$LITE_RADAR" "alpha:false,desynchronized:true" "opaque low-latency visible Lite canvas"
need "$LITE_RADAR" "radarLiteBaseConcurrency())" "bounded parallel OSM tile pool"
need "$LITE_RADAR" "radarLiteRadarConcurrency())" "bounded parallel RainViewer tile pool"
need "$LITE_RADAR" "radarLiteAvailableFrameCount()===2" "Play unlock after second completed frame"
need "$LITE_RADAR" "createImageBitmap" "Lite ImageBitmap compositor"
need "$LITE_RADAR" "frame.close" "Lite ImageBitmap close"
need "$LITE_RADAR" "radarLoadDetachedImage(radarBaseTileURL(slot),token)" "Lite OSM base tiles"
need "$LITE_RADAR" "await radarYield();" "Lite time-budgeted draw yield"
need "$RADAR" "if(radarIsLite())return;" "Lite prefetch hard stop"
need "$LITE_RADAR" "function radarSetLiteControls" "Lite controls"
need "$LITE_RADAR" "function radarCancelLiteRequests" "Lite detached-image cancellation"
need "$LITE_RADAR" "coverage.drawn!==coverage.planned" "Lite complete-coverage replacement guard"
need "$LITE_RADAR" "complete radar coverage is unavailable" "Lite partial-map retry status"
need "$RADAR" "RADAR_LITE_EDGE_OVERSCAN_PX" "Lite measured tile edge overscan"
need "$HEALTH" "function nightDimOpacity" "pointer-overlay night dim calculation"
need "$HEALTH" 'document.body.style.removeProperty("filter")' "legacy body-filter cleanup"
absent "$HEALTH" "document.body.style.filter=" "whole-page night dim filter write"
need "$ROOT/index.html" 'id="nightdim"' "permanent night dim overlay markup"
need "$ROOT/ui/css/dashboard/base.css" "pointer-events:none" "night dim touch transparency"
need "$MSG_OVERLAY" "function popupOpenTransaction" "first-paint popup transaction"
need "$MSG_OVERLAY" "function popupInvalidateWork" "popup stale-work invalidation"
need "$DAY_POPUP" "function dtBuildDayModel" "cached day popup model"
need "$DAY_POPUP" "state.views" "cached Timeline/List views"
need "$DAY_POPUP" "const DT_CARD_STAGE_CHUNK=16" "bounded static timeline-card staging"
need "$DAY_POPUP" "function dtStageTimelineCards" "one-time timeline card staging"
need "$DAY_POPUP" "function dtCancelTimelineStage" "timeline staging cleanup on view swap/close"
need "$DAY_POPUP" "dtLiteDayPopupProfile" "Lite List-first timeline default"
absent "$DAY_POPUP" "function dtWindow" "scroll-driven timeline virtualizer"
absent "$DAY_POPUP" 'addEventListener("scroll"' "timeline scroll-time listener"
need "$DAY_POPUP" "body._dtUserScrolled" "user-scroll-first timeline guard"
absent "$DAY_POPUP" "dt-event-stripe" "per-event timeline stripe node"
absent "$DAY_POPUP" "getBoundingClientRect" "first-touch timeline rectangle reflow"
need "$ROOT/ui/js/control-lazy-loader.js" "function showCtrlLoadingShell" "immediate Control loading shell"
need "$ROOT/ui/js/control-lazy-loader.js" "function scheduleControlAssetWarmup" "idle Control asset warmup"
need "$ROOT/ui/js/control-api.js" "function ctrlAbortPageRequests" "page-scoped Control cancellation"
need "$THEME_RUNTIME" "dashboard-theme-vars" "atomic theme variable stylesheet"
need "$THEME_RUNTIME" "function dashboardRuntimeSettings" "early-safe runtime settings bridge"
need "$THEME_RUNTIME" "scheduleSeasonalDecorReconcile" "deferred seasonal decor reconciliation"
absent "$THEME_RUNTIME" "seasonalDecorSignature" "early theme path reaches later lexical settings"
need "$SETTINGS" "function syncDashboardRuntimeSettings" "settings bridge synchronization"
need "$SEASONAL_DECOR" "_SEASONAL_DECOR_RECONCILE" "bounded deferred decor scheduler"
need "$SEASONAL_DECOR" "function reconcileSeasonalDecor" "decor-only reconciliation"
absent "$SEASONAL_DECOR" "(SETTINGS&&SETTINGS.seasonalDecor)" "pre-settings direct settings read"
need "$ROOT/index.html" "id=\"radarlitecanvas\"" "Lite radar canvas markup"
need "$ROOT/index.html" "id=\"radarrefresh\"" "Lite manual refresh control"
need "$ROOT/index.html" "id=\"radarzoom\"" "Lite zoom control"
need "$TOUCH_CSS" "#radarfull.radar-lite #radarbase" "Lite hides tile/base layers"
need "$TOUCH_CSS" "height:100%;width:auto;max-width:100%" "Lite canvas fills stage short dimension"
need "$TOUCH_CSS" "padding:6px" "Lite stage uses tight padding"
absent "$TOUCH_CSS" "width:min(100%,512px)" "old Lite canvas display cap"
need "$TIMELINE_CSS" "z-index:30" "timeline view controls remain above scrolling event cards"
need "$TIMELINE_CSS" "top:0" "timeline view controls use an in-scroll sticky top"
need "$TIMELINE_CSS" "box-shadow:0 1px 2px" "cheap timeline card shadows"
absent "$TIMELINE_CSS" "content-visibility" "timeline deferred-content placeholders"
absent "$TIMELINE_CSS" "contain-intrinsic-size" "timeline placeholder sizing"
absent "$TIMELINE_CSS" "dt-event-stripe" "timeline stripe paint CSS"
absent "$TIMELINE_CSS" "0 6px 18px" "broad timed-card shadow"
absent "$TIMELINE_CSS" "0 4px 14px" "broad all-day/list-card shadow"
need "$TOUCH_CSS" "object-fit:fill" "square radar tile rendering"
need "$TOUCH_CSS" "#radarbuttons" "separate large manual radar controls"
need "$ROOT/index.html" "id=\"radarnow\"" "Now radar control"
need "$ROOT/kiosk.sh" "WEBKIT_DISABLE_COMPOSITING_MODE=1" "Lite software compositing"
need "$ROOT/kiosk.sh" "MALLOC_ARENA_MAX=2" "Lite allocator arena cap"
need "$ROOT/kiosk.sh" "MALLOC_TRIM_THRESHOLD_=131072" "Lite allocator trim threshold"
need "$ROOT/bin/dashboard-lite-session.sh" "session_is_lite_profile" "Lite session profile detection"
need "$ROOT/bin/dashboard-lite-session.sh" "G_SLICE=always-malloc" "Lite session allocator setting"
need "$TOUCH_CSS" "background:#0d161f" "neutral Lite letterbox background"
need "$DEMO" "func (a *app) clearDemoMode" "non-seeding demo clear helper"
need "$DEMO" 'fs.Bool("clear"' "clear demo CLI flag"
need "$INSTALLER" "--setup-demo-mode --clear" "installer uses non-seeding demo clear"
need "$INSTALLER" "Offline, verified Dash-Go uninstall" "offline uninstall contract"
need "$INSTALLER" "remove_dashboard_cron_jobs" "Dash-Go-only cron cleanup"
need "$INSTALLER" "remove_make_archive" "verified uninstall archive"
need "$INSTALLER" "REMOVE_SENTINEL" "external uninstall sentinel"
need "$INSTALLER" 'if [ "$REMOVE_MODE" = "1" ]; then run_remove_install; exit $?; fi' "remove dispatch before GitHub Release work" "remove dispatch before GitHub Release work"
absent "$INSTALLER" "pkill -x surf >/dev/null" "generic Surf kill from uninstall"
need "$ROOT/kiosk.sh" "DASH_REMOVE_SENTINEL" "kiosk uninstall sentinel"
need "$ROOT/bin/dashboard-lite-session.sh" "SESSION_REMOVE_REQUESTED" "session exits instead of restarting on uninstall"
need "$GUARD" "INFO+=(\"post-update-runtime-verifier-started\")" "post-update verifier audit info"
need "$GUARD" "WARNINGS=()" "guard warning classification"
absent "$GOHEALTH" "onlyLegacyPostUpdateVerifierNotice" "retired legacy post-update guard suppression"
need "$INSTALLER" "def sha256_file(path):" "streaming manifest verifier"
need "$INSTALLER" "fh.read(1024 * 1024)" "bounded manifest hash chunk"
need "$INSTALLER" "DASH_MANIFEST_VERIFY_VMEM_KB" "bounded manifest verifier memory"
absent "$INSTALLER" "hashlib.sha256(open(path, 'rb').read())" "whole-file manifest hashing"
need "$GOHEALTH" "func (s *Service) clockTrustworthy" "clock confirmation/floor self-validation"
absent "$GOHEALTH" "Year() < 2024" "hardcoded clock-year floor"
need "$ROOT/bin/dashboard-resilience-lib.sh" "clock-confirmed.json" "persistent NTP confirmation cache"
need "$ROOT/bin/dashboard-resilience-lib.sh" "buildEpoch" "dynamic installed-build clock floor"
need "$ROOT/bin/doctor.sh" "dash_clock_verified" "Doctor shared clock trust helper"
need "$GOHEALTH" "lastSuccessAt" "data health reads successful refresh timestamps"
need "$GOHEALTH" "displaySleepState" "display-sleep-aware data health"
need "$HEALTH" "staleSleepGrace" "browser display-sleep stale grace"
need "$RADAR" "function radarNextFrame" "first-paint layout settle"
need "$RADAR" "function radarStageReady" "transitional radar stage guard"
need "$GOHEALTH" 'case "pending", "running", "recovering":' "silent post-update in-progress state"
need "$HEALTH" 'dev==="failing"||dev==="degraded"' "actionable-only device pill states"
need "$HEALTH" "Math.max(10,wxMin*2,calMin*2)" "cadence-aware first-boot stale grace"
need "$WEATHER_DOMAIN/radar_config.go" '.dashboard-radar.env' 'private radar env reader'
need "$WEATHER_DOMAIN/radar_tile.go" 'radarTileURL' 'radar tile proxy'
need "$WEATHER_DOMAIN/radar_tile.go" 'radarTileURL' 'known-provider validation'
need "$WEATHER_DOMAIN/radar_tile.go" 'radarAllowRequest' 'per-provider rate limiting'
need "$ROOT/internal/platform/diagnostics.go" 'RADAR_XWEATHER_ID' 'explicit radar credential redaction'
need "$ROOT/internal/platform/diagnostics.go" 'KEY|SECRET|TOKEN|PASS|PASSWORD' 'provider credential suffix redaction'
need "$MESSAGES_DOMAIN/messages_providers.go" 'decodeMessageJSON' 'bounded message JSON decoder'
need "$MESSAGES_DOMAIN/messages_providers.go" '2<<20' 'general message provider body cap'
need "$MESSAGES_DOMAIN/messages_providers.go" '1<<20' 'random-word provider body cap'
need "$SERVER/config_backup_restore.go" 'maxConfigBackupEntryBytes' 'config restore per-entry ceiling'
need "$SERVER/config_backup_restore.go" 'maxConfigBackupTotalBytes' 'config restore total ceiling'
need "$SERVER/config_backup_restore.go" 'stageConfigBackup' 'config restore staging before mutation'
need "$SERVER/config_backup_restore.go" 'replaceRestoreTrees' 'authoritative config/calendar replacement'
need "$MESSAGES_DOMAIN/service.go" 'messageWriteMu' 'serialized message cache writes'
need "$MESSAGES_DOMAIN/messages_feeds.go" 'updateMessageState' 'serialized message cache mutation helper'
need "$ROOT/bin/doctor.sh" '.dashboard-radar.env' 'Doctor radar-env permissions check'
need "$INSTALLER" '.dashboard-radar.env' 'installer radar env handling'
need "$INSTALLER" 'DO_RADAR' 'installer radar stage'
need "$RECUR" 'eventDate(ev, dt.Year(), dt.Month(), dt.Day(), 23, 59, 59, 999999999)' 'inclusive date-only RRULE UNTIL boundary'
need "$RECUR" 'calendarDayDiff' 'DST-safe recurrence backfill arithmetic'
need "$RECUR" 'weeklyOccurrencesBefore' 'COUNT-aware weekly fast-forward'
need "$ICS" 'ExdateDays' 'date-only EXDATE day-key fallback'
need "$ICS" 'RecurIDDateOnly' 'date-only RECURRENCE-ID day-key fallback'
need "$RECUR" 'func recurrenceDayKey' 'calendar-day recurrence key'
absent "$EVENT_UTILS" 'func max(' 'custom max shadow removed for Go builtin'
need "$WEATHER_HTTP" 'weatherJSONResponseLimit' 'bounded weather JSON response limit'
need "$WEATHER_HTTP" 'io.LimitReader' 'bounded keyed weather decoder'
need "$WEATHER_HTTP" 'weatherHTTPClient' 'dedicated bounded weather client'
need "$OPENMETEO" 'io.LimitReader' 'bounded Open-Meteo decoder'
need "$ISS" 'context.WithTimeout' 'bounded ISS request timeout'
need "$CALENDAR_SERVICE" 'HTTPClient' 'injected bounded ISS client'
need "$SETTINGS_GO" 'func (a *app) updateSettings' 'serialized settings read-modify-write helper'
need "$SETTINGS_SERVICE" 'writeMu' 'dedicated settings write serialization lock'
need "$BLEND" 'browser-authoritative normalized sources' 'single authoritative browser weather blend'
absent "$BLEND" 'func blendCurrentGo' 'retired divergent server weather blend'
need "$WEATHER_DOMAIN/weather_fetch.go" 'wg.Go' 'Go 1.25 WaitGroup.Go weather workers'
need "$WEATHER_DOMAIN/radar_tile.go" 'make([]time.Time' 'non-aliasing radar request pruning'

node --check "$TAP"
node "$ROOT/tests/tap-primitive-regression-smoke.mjs"
node "$ROOT/tests/control-card-open-smoke.mjs"
node "$ROOT/tests/close-controls-smoke.mjs"
node --check "$RADAR"
node --check "$LITE_RADAR"
node --check "$WEATHER"
node "$ROOT/tests/moon-phase-svg-smoke.mjs"
node "$ROOT/tests/weather-current-metrics-smoke.mjs"
node --check "$APP_LAUNCHER"
node "$ROOT/tests/theme-bootstrap-smoke.mjs"
node "$ROOT/tests/day-timeline-static-smoke.mjs"
node "$ROOT/tests/day-timeline-static-runtime-smoke.mjs"
node "$ROOT/tests/day-timeline-lite-paint-smoke.mjs"
node "$ROOT/tests/scroll-contract-smoke.mjs"
node "$ROOT/tests/control-profile-custom-smoke.mjs"
node "$ROOT/tests/control-profile-package-contract-smoke.mjs"
node "$ROOT/tests/control-profile-relocated-settings-layout-smoke.mjs"
node "$ROOT/tests/control-profile-message-timing-layout-smoke.mjs"
node "$ROOT/tests/message-fade-contract-smoke.mjs"
node "$ROOT/tests/adaptive-cache-budget-smoke.mjs"
node "$ROOT/tests/platform-boundary-smoke.mjs"
node "$ROOT/tests/control-1080p-layout-smoke.mjs"
node "$ROOT/tests/control-centered-rail-layout-smoke.mjs"
node "$ROOT/tests/health-warning-pill-smoke.mjs"
node "$ROOT/tests/app-launcher-smoke.mjs"
echo 'PASS: App Launcher plus retained dashboard startup, touch, observed-history radar, static staged day timeline, dim, popup, health, uninstall, calendar, fetch, settings, focused Performance Profile plus feature-local Calendar/Display controls, automatic message timing/fade, health-warning silence, and adaptive Lite cache contracts are present and syntactically valid'

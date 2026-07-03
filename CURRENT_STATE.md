# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release

- **Version:** `1.5.5`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, source archive, SPDX SBOM, and `SHA256SUMS`.
- **Release integrity:** published assets use immutable GitHub Releases; installation and update flows validate downloaded and staged content before managed files are replaced.

## Current development beta

- **Version:** `1.5.6-beta.8`
- **Track:** beta
- **Baseline:** `1.5.5` stable, preserving its responsive dashboard and Showcase Studio release workflow unchanged.
- **Focus:** make private Google, iCloud, and CalDAV calendars intentionally discoverable, individually selectable, and safely manageable as exact two-way sources without adding a second remote-sync engine.

## 1.5.6-beta.8 highlights

- **Discover before selecting:** Calendar Manager now offers a user-triggered, disposable vdirsyncer discovery inventory. It shows remote Google/iCloud/CalDAV collections without changing the active configuration, dashboard mirror, event cache, cron schedule, or write permissions. Nothing becomes active until the user explicitly adds it.
- **Exact selected sources:** each selected remote collection gets a separate generated pair, local vdir key, dashboard mirror, provider label, and per-source sync state. Opaque remote IDs remain metadata rather than filesystem names, so duplicate display names and URL-like CalDAV identifiers stay safe and distinct.
- **Manageable two-way calendars:** selected calendars can be changed later between display-only and editable from Dashboard Control. Supported normal event writes remain local-first and now queue only the affected vdirsyncer pair instead of every private calendar. The global calendar-edit safety switch and deletion PIN protections remain in force.
- **Conflict-safe migration:** new generated pairs remove remote-wins conflict configuration. A detected conflict keeps both versions intact and marks that source as needing attention. Existing broad discovery mirrors are preserved as read-only during migration; an update refreshes only derived local private-calendar configuration and never contacts a provider.
- **Live popup capabilities:** normal eligible private-calendar events now retain static writeback candidacy in the local event cache while their popup checks the current local writeback registry when opened. Enabling a selected calendar turns on the master Dashboard-edit guard, refreshes event capabilities, and restores `+ Add event` plus eligible Edit/Delete actions without requiring a reboot or waiting for a periodic cache pass.
- **One Calendar Manager scroll surface:** Calendar Manager groups now extend the normal Calendars Control page rather than creating nested scroll panes. Focused setting updates preserve the visible calendar row, and a swipe beginning on a Calendar Manager action cancels the action instead of firing it. A selected editable calendar missing its local registration now reports **Needs attention** and offers a non-destructive repair path.

- **Corrected beta.8 source rebuild:** Replaced a brittle event-domain boundary assertion that depended on `gofmt` column spacing with a Go AST contract that verifies the same `ServiceConfig` seams. This corrects a false build failure without changing runtime behavior.

## 1.5.6-beta.6 highlights

- **Gentle private-calendar work:** every private-calendar synchronization path—cron, manual run, setup pull, and queued Dashboard writeback—re-execs through Dash-Go’s existing low-priority helper. Vdirsyncer, local merge work, and event-cache regeneration inherit lower CPU and I/O priority without introducing a daemon or retained Python process.
- **Setup-time discovery only:** remote collection discovery now runs during explicit setup/refresh and one-time Google authorization, not every 15-minute routine sync. Existing exact collections continue to sync normally; rerun private-calendar setup to discover a newly created remote collection.
- **Bounded schedule preserved:** calendars still synchronize one at a time with the existing shared lock and per-pair fault isolation, so a slow, failed, or unauthorized provider cannot stack work or block another private calendar.

## 1.5.6-beta.5 highlights

- **Pinned private-calendar toolchain:** setup now manages one owner-only pipx environment under `~/.dashboard-vdirsyncer/`, installing `vdirsyncer[google]` at exact version `0.20.0`. Debian-family devices use APT only to obtain pipx when needed; Dash-Go does not use raw `pip --user`, modify the system Python environment, or run automatic pipx upgrades.
- **Known executable and migration:** generated private-calendar sync wrappers call the exact Dash-Go-managed vdirsyncer path rather than whichever executable happens to be on `PATH`. Reopening setup and finishing without a new connection safely regenerates existing private-calendar configuration and wrappers without rewriting credentials or remote data.
- **Provider consistency:** CalDAV and Google now receive the same installed vdirsyncer environment and Google OAuth extra from the first install. Source smokes cover exact-version install, attempted pipx pinning, owner-only paths, no raw-pip/system-vdirsyncer installation, and generated-wrapper path ownership.

## 1.5.6-beta.4 highlights

- **Google Calendar writeback:** setup can register Google Calendar through vdirsyncer’s OAuth-backed `google_calendar` storage. An exact, materialized Google Calendar ID may opt into the same local-first create/edit/skip/delete controls as an enrolled private CalDAV collection; a blank or broad discovery remains display-only.
- **Private OAuth handling:** client secrets and token files remain outside `~/dashboard` in owner-only vdirsyncer paths. One-time authorization is explicit, supports local-browser, SSH port-forward, or desktop-token-copy workflows, and never runs from cron.
- **Independent scheduled sync:** each eligible private calendar pair is bounded and synchronized separately. Missing/revoked Google authorization and a failed remote pair retain that pair’s previous dashboard mirror while other enrolled CalDAV or Google calendars continue syncing. Existing eight-field CalDAV pair records remain compatible.
- **Read-only boundaries preserved:** website and URL ICS subscriptions, generated feeds, unmanaged local files, broad mirrors, attendee/organizer events, and detached recurrences remain structurally excluded from every writeback route.

## 1.5.6-beta.3 highlights

- **Private two-way CalDAV calendars:** Dashboard edits are opt-in and eligible only for an exact, setup-registered local vdir collection. A user can add a standard event from a day popup, edit a simple event in its original collection, or skip one recurring occurrence. Each accepted change saves to the local vdir first, refreshes Dash-Go’s derived mirror, then queues normal CalDAV synchronization.
- **Strict read-only boundaries:** website/URL ICS subscriptions, local unmanaged files, broad multi-collection mirrors, generated Dash-Go feeds, attendee/organizer events, and detached recurrence instances cannot enter any writeback route. Calendar Manager protects registered remote mirrors from delete/trash actions; it can hide them but never delete a remote calendar.
- **Safe event semantics:** writeback preserves nested alarms and unknown provider properties, rejects aggregate items, writes recurrence exceptions in the master DTSTART’s date/time/TZID form, keeps all-day dates date-based with an exclusive end date, and requires an enabled Dashboard Control PIN for one-time event deletion.
- **CalDAV control and status:** setup can enroll one exact private CalDAV collection for dashboard edits, while broad discovered collections remain display-only. Calendars Control shows writeback availability and enables or disables each registered collection without exposing remote collection paths.

## 1.5.6-beta.2 highlights

- **Frontend first-paint and steady-state efficiency:** runtime fonts now revalidate instead of being re-downloaded and re-parsed at every kiosk launch; tap-binding cleanup and last-known-event snapshots move off layout-critical work; and calendar day-event fitting batches its write/read/write phases across cells to avoid per-day forced reflows.
- **Small kiosk polish:** the dashboard supplies an explicit empty favicon to prevent the browser’s avoidable `/favicon.ico` 404 request. These are timing and caching changes only; dashboard content and interaction behavior remain unchanged.

## 1.5.6-beta.1 foundation

- **Update and repair reliability:** server SIGTERM/SIGINT now performs a bounded graceful HTTP shutdown; explicit GitHub-release repair resolution no longer loses its result when the installed `VERSION` is damaged or missing; and interrupted-update recovery preserves its truthful recovery timestamp.
- **To Do delivery paths:** the intentionally bounded 75-second inbound sync receives a response-specific write deadline, while its SSE stream clears only its own deadline and sends a lightweight heartbeat to avoid unnecessary EventSource reconnects.
- **Durability and security compatibility:** atomic file and JSON writes flush content, requested mode, and replacement directory metadata in durable order; PIN derivation now uses Go’s standard PBKDF2 while a regression proves legacy hashes remain byte-compatible; four-to-eight ASCII-digit PIN compatibility remains unchanged.
- **Maintenance and privacy:** removed dozens of dead startup regexes, replaced stale versioned outbound User-Agent strings, rendered safe map-fallback reasons, and removed a committed runtime JSON artifact while moving shared tests to their intended temporary Todo data directory.

## 1.5.5 highlights

- **Responsive showcase rendering:** the dashboard now renders cleanly at 1920×1080, 1366×768, and 1280×800 landscape and 1080×1920, 800×1280, and 768×1024 portrait. Narrow day columns place event times on their own line with word-boundary wrapping and hyphenation, portrait walls show abbreviated weekday headers so all seven days remain visible, the sidebar clock can no longer overflow its panel, and scrolling panes fade at their edges instead of cutting text mid-glyph.

## 1.5.4 highlights

- **Dashboard Control:** consolidated competing Control styling into clear ownership, restored visible selected and pressed touch states, stabilized the six-tab rail, renamed the household/preferences tab to Settings, and made action and option grids deliberate rather than auto-fit accidents. Five choices balance as centered 3 + 2 and six as 3 + 3, while quick-action cards retain their intrinsic height.
- **Installer and setup:** restored the advertised Control PIN, dashboard-service, and SSH menu actions; made Demo Mode safe by default; corrected the moon-phase Control gesture; retained current PIN-duration choices on Enter; moved pre-flight after a real menu action; and made menu identities and setup guidance self-consistent.
- **PIN, Doctor, and repair hardening:** made verifier semantics strict, configuration reads fail closed, every-open Control sessions server-enforced, and Control/inbox lockouts persistent and escalating. Doctor repair selection now matches what it renders, redirected fixes are safe-only, and repair preserves backups outside the application tree with a verified recovery recipe for broken server binaries.
- **Weather and messages:** normalized daily precipitation to millimetres at every active adapter, retained one authoritative browser blend with daily low/high coherence, and added privacy-preserving message-fit diagnostics without rotation-time layout work.
- **Generated calendars and schedules:** fixed every-N-days chore cadence across DST, retained prior good ISS feeds on provider failures, hardened one-time occurrence moves and stale overrides, removed duplicate clamped month-end paydays, improved holiday shifts, and made seasons, February 29 celebrations, and moon-output reporting more truthful.

## 1.5.3 highlights

- **Calendar décor:** every seasonal, holiday, and calendar-aware observance theme carries five static calendar decals. User-selected décor density stays available on every profile, including Lite, without filters, masks, gradients, animation, polling, or external assets.
- **Visual polish:** Bold and High Contrast weather SVGs are clearer, and crescent earthshine receives a modest refinement while retaining the existing rendering model.

## 1.5.2 highlights

- **Message readability:** rotating messages use conservative Lite fitting, bounded rendered verification, and safe ellipsis within the fixed footer. The refreshed household-safe catalog preserves hidden and edited built-in-message state while using loaded calendar events for appropriate observance wording.
- **Household scheduling:** Dashboard Control manages named Payday, Trash Pickup, and Recycling Pickup rules without rerunning setup. Dash-Go-owned schedule occurrences can be moved, skipped, or restored from the day popup without making subscribed calendars editable.
- **Curated themes:** the 100-theme picker has purposeful categories rather than a More catchall. Seasons stay four columns; other groups use touch-safe four-to-six-column grids. Hanukkah and Kwanzaa remain local-cache-only observance themes and appear only when their matching configured calendar source supplies a recognized current event.
- **Correctable day actions:** Dash-Go-owned Chore Wheel, Maintenance, and Routine checkboxes can undo a mistaken current or past completion when the underlying record can safely return to its prior state. Future, skipped, external, and unsafe-after-later-change items remain protected.
- **On-demand hardening:** weather cache markers, backups, calendar-link restoration, fonts, control previews, and local fallbacks were tightened without adding dashboard-startup work, polling, timers, extra network calls, or periodic filesystem scans. Calendar-link backups support trusted targets under the dashboard user’s home and `/Calendars`.

## Recommended operating model

- **Fresh base:** Raspberry Pi OS Lite written with Raspberry Pi Imager, with hostname, normal user, localisation, network, and SSH configured before first boot.
- **Runtime:** the Go dashboard control server listens on loopback and Surf/WebKit displays the kiosk interface.
- **Primary reliability target:** Raspberry Pi Zero 2 W using the Lite profile.
- **Managed application tree:** `~/dashboard`.

## Product posture

- Dashboard Control opens with Device status ready for a glance; other cards remain collapsed and lazy.
- Household apps are local-first, touch-first, and loaded only when opened.
- Provider connections are optional enhancements. Local calendar, list, message, and household workflows remain useful while offline or unlinked.
- Weather radar is on-demand; Lite keeps its work and retained visual state bounded.
- The shared on-screen keyboard starts Shift-active, supplies the focused field’s affirmative action, and reserves and releases its own layout space cleanly.
- The Family Message Board maintains normal form scrolling while the on-screen keyboard is open without letting a native scrollbar draw above it.

## Operational guarantees

- Installation and updates stage, validate, and atomically replace managed files while preserving user configuration, calendars, app data, secrets, and household history.
- Normal updates restart the local service, confirm readiness, and return the kiosk to Dash-Go rather than the login screen.
- Doctor and repair distinguish narrow application-file recovery from wider service, kiosk, scheduler, and package recovery.
- Lite work remains bounded across memory, network activity, DOM lifecycle, background jobs, and browser recovery.
- Visible **Check soon** notices remain reserved for actionable risk rather than expected post-update or normal background recovery behavior.

## Current implementation boundaries

- `cmd/dashboard-control-server` owns process startup, route and CLI composition, lifecycle wiring, and release orchestration.
- `internal/*` packages own their domain state, persistence, bounded services, locks, and domain-specific behavior.
- `internal/release` owns GitHub release version parsing, track selection, public-asset validation, and release metadata resolution.
- Internal packages do not import `package main`, and cross-domain coordination uses narrow ports or callbacks instead of a whole-application dependency.
- Browser source order is manifest-owned; browser bundles and compiled binaries are generated by the local release builder rather than hand-edited source.

## Documentation and release discipline

- `README.md` is the user-facing setup and operating guide.
- `CHANGELOG.md` records concise stable-release history.
- `INTEGRATIONS.md` documents optional outside services and their local/offline behavior.
- `PRIVACY.md` documents local storage, optional network sharing, backups, and administrator responsibilities.
- `THIRD_PARTY_NOTICES.md` records distributed third-party software and asset notices.
- `AI.md` contains durable assistant guidance only and is intentionally excluded from source handoffs and release assets.
- The local Windows/WSL release builder remains authoritative for generated assets, compiled binaries, package validation, checksums, SPDX SBOM generation, and GitHub Release asset preparation.

This file is an immediate operating snapshot. Do not add beta journals, completed project history, detailed test logs, or prior-release issue lists here.

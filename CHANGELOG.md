# Dash-Go Changelog

This changelog records stable Dash-Go milestones. Detailed development increments are consolidated at stable promotion so the file remains useful as a product history rather than a release-by-release development journal.

## [1.5.6-beta.10] — 2026-07-03

### Truthful private-sync outcomes and recurring-edit correctness

- Corrected private-calendar sync reporting end to end. The generated wrapper now captures each pair's own vdirsyncer output, classifies the outcome (synced, conflict, emptied-collection guard, missing discovery, rejected credentials), reports it as structured `RESULT` lines to the dashboard's queued sync, and records it in a durable per-pair results file. Previously all vdirsyncer output went only to the log file, so a real conflict was reported to the household as a generic "will retry automatically" and the conflict state was unreachable.
- Fixed Control-selected calendars never synchronizing: activation now performs the one bounded, noninteractive vdirsyncer discovery its new exact pair requires before the initial targeted sync. Without it, vdirsyncer refused every subsequent sync of a calendar added through Calendar Manager until interactive setup was re-run over SSH.
- Fixed deleting a calendar's final event permanently wedging its sync on vdirsyncer's emptied-storage guard. A dashboard-initiated delete now carries a one-run `--allow-empty-once` permission that maps to `--force-delete` for exactly that targeted sync; an unexpectedly emptied collection remains blocked and is reported as needing attention instead of retrying forever.
- Fixed `setup-vdirsyncer.sh --refresh` (which runs automatically after updates) silently re-enabling per-calendar Dashboard edits the user had turned off: the previous registry flags are now read with a JSON parser instead of format-sensitive text matching against the server's indented registry.
- Corrected recurring series edits to re-render `EXDATE` exclusions onto the same civil days in the new start time, zone, and value form. Previously a series time change orphaned every exclusion and silently resurrected skipped occurrences on the dashboard and on provider clients; the series writer also now enforces the simple-rule boundary itself.
- Corrected detached-occurrence matching to compare parsed instants rather than exact text, so a provider-written UTC-form `RECURRENCE-ID` over a TZID master is revised in place instead of duplicated, and an existing `RANGE=THISANDFUTURE` override now refuses the edit instead of gaining a conflicting sibling.
- Calendar event form: an all-day event now displays its inclusive last day and converts to the exclusive contract on save, an edit keeps targeting its own calendar when the cached event carries only `calUrl`, and buttons gained the dashboard's standard press feedback.
- Integrated cron-originated private-calendar outcomes into Dashboard Control status, using a bounded and strict per-pair result parser so a newer background conflict, missing-discovery, authorization, empty-collection guard, or failure state appears on the affected calendar row without exposing raw vdirsyncer output or provider paths.
- Added scoped recovery controls for exact selected sources. **Repair connection** runs one targeted discovery and sync for an `attention-undiscovered` pair without broad account discovery or selection changes. A conflict exposes PIN-gated, themed one-shot choices to retain the remote or Dashboard side for that one pair; normal configurations remain conflict-safe, temporary resolver configuration is removed on every exit path, and a bounded owner-only local snapshot is taken before a remote-winner run.
- Hardened recurring exceptions: Skip refuses an occurrence that already has a detached override, new overrides inherit the master `SEQUENCE`, simple-series forms warn when retained excluded dates may need review after a date move, and vdir lookup prefilters nonmatching items before strict UID parsing while retaining duplicate-UID failure behavior.
- Made generated shell mirrors match the Go mirror path's atomic CRLF and `0644` output contract; moved shared popup/OSK spacing to the common popup stylesheet; replaced the calendar writeback error note's `color-mix()` dependency with existing theme tokens; and added secret-safe Google versus iCloud/CalDAV authorization guidance.

## [1.5.6-beta.9] — 2026-07-03

### Recurring private-calendar management and truthful updates

- Added a scoped **Manage recurring event** flow for exact editable Google, iCloud, and compatible CalDAV sources. An eligible recurring event can now update one occurrence through a detached `RECURRENCE-ID` exception, or update a deliberately constrained series while retaining its existing repeat rule, exclusions, alarms, and unknown provider properties. Existing exceptions, attendees/organizers, `RDATE`, advanced selectors, and ambiguous provider-managed series remain explicitly read-only with an explanation.
- Replaced the one-time-event-only primary wording with **Manage event** and added clear **This occurrence**, **Entire series**, and **Skip this occurrence** actions. Each accepted change remains local-first, refreshes only the affected mirror/cache, and queues only the selected vdirsyncer pair.
- Added strict normal-update planning for SSH and Dashboard Control. Equal releases now exit as a successful no-op, older selected releases never downgrade the device, and the Dashboard API rechecks eligibility before it creates a backup, job, action-history row, systemd request, download, replacement, or browser restart. Explicit repair retains its intentional same-version recovery behavior.
- Removed the unrelated Open-Meteo internet probe from private-calendar preflight. Calendar setup now verifies connectivity at the actual APT, pipx, Google OAuth/CalDAV, iCloud, or selected CalDAV operation and describes that operation rather than declaring the whole device offline.
- Added recurrence writer, server capability, installer no-op, update-card, and source-contract regressions; preserved beta.8’s exact-selection, live capability, and one-scroll-root Calendar Manager behavior.

## [1.5.6-beta.8] — 2026-07-02

### Selected private calendars and safe two-way management

- Added a user-led **Discover available calendars** flow in Dashboard Control. Discovery runs in a disposable vdirsyncer workspace, returns an inventory only, and never alters active pairs, mirrors, event cache, cron, visibility, write permissions, or remote calendars.
- Added explicit per-collection activation: each chosen Google, iCloud, or compatible CalDAV collection receives a generated exact vdirsyncer mapping, a safe generated local collection key, and its own Dashboard source. Broad legacy discovery mirrors remain read-only and are retained rather than silently replaced.
- Added selected-calendar management controls to change a source between display-only and editable, queue a sync for that one collection, or stop future sync without deleting the provider calendar or the preserved local mirror.
- Changed local-first writeback to queue the affected generated pair rather than every private calendar, with serial coalescing, per-calendar status, and conflict-safe behavior. New private pairs no longer use remote-wins conflict resolution; a conflict leaves both sides intact and surfaces a needs-attention state.
- Added migration refresh after an update, isolated-discovery, exact-selection, edit-permission, targeted-sync, and private-calendar installer smokes. Updated Google documentation to describe the actual vdirsyncer CalDAV/OAuth route, collection selection, and two-way acceptance expectations.
- Corrected Calendar Manager edit-state behavior: adding or enabling an exact private calendar now activates the master Dashboard edit guard, refreshes local event capabilities, and evaluates current local permissions when an event popup opens. Eligible normal one-time events regain Edit/Delete actions, and full-day popups regain `+ Add event`, without relying on an old cache record.
- Reworked Calendar Manager into one continuous Calendar-page scroll surface. Removed nested bounded manager panes, preserve the affected row across focused updates, suppress a button action after a real page swipe, and show a **Needs attention** repair action when a selected editable source has lost its local writeback registration.

- Corrected the beta.8 source boundary test so it verifies the event-service policy seam through Go syntax rather than fragile formatter-dependent spacing; runtime behavior is unchanged.

## [1.5.6-beta.6] — 2026-07-02

### Lower-impact private-calendar synchronization

- Routed every generated private-calendar sync invocation through Dash-Go’s existing `dashboard-lowprio.sh` helper. Scheduled, manual, setup, and queued writeback runs now give Surf/WebKit and the dashboard server lower CPU/I/O contention while preserving the single shared sync lock and one-pair-at-a-time behavior.
- Moved remote collection discovery out of the 15-minute sync wrapper and into explicit private-calendar setup/refresh plus the one-time Google authorization flow. Routine syncs now execute only the already-configured exact pairs; reopening setup discovers newly created or deliberately changed remote collections.
- Added a functional scheduling smoke that proves setup discovers a pair once, normal syncs do not rediscover it, and each wrapper invocation re-execs through the low-priority helper exactly once.

## [1.5.6-beta.5] — 2026-07-02

### Pinned private-calendar synchronization

- Standardized optional CalDAV and Google private-calendar synchronization on one Dash-Go-owned `pipx` environment pinned to `vdirsyncer[google]` 0.20.0. Raspberry Pi OS/Debian/Ubuntu may install `pipx` through APT, but the vdirsyncer application never enters system Python and Dash-Go no longer offers raw `pip --user` or a distro-version-dependent vdirsyncer path.
- Generated sync wrappers now call the known owner-only wrapper beneath `~/.dashboard-vdirsyncer/bin/`, preventing a later PATH change or unrelated system package from silently changing the sync executable. Pipx pinning is attempted where supported; exact installation and the absence of automatic upgrades remain the durable baseline.
- Made existing private-calendar setups migratable: reopen setup and finish with no new calendar to regenerate the config/wrapper around the pinned tool without touching saved credentials, tokens, calendar mappings, or remote data. Added a functional pipx-policy smoke alongside the CalDAV and Google setup smokes.

## [1.5.6-beta.4] — 2026-07-02

### Two-way Google Calendar sync

- Added a Google Calendar provider to the CalDAV setup flow using vdirsyncer's OAuth-based `google_calendar` storage over Google's CalDAV endpoint. Google calendars ride the existing pull/merge/writeback pipeline unchanged: exact enrolled collections may opt into local-first Dashboard add/edit/skip with background push, while broad discovered mirrors stay read-only.
- OAuth material never enters the dashboard webroot: the client secret is stored beside CalDAV app passwords and fetched by command, and the token lives in a new owner-only `google-tokens` directory. Setup performs the one-time authorization interactively with explicit headless/SSH port-forward guidance; declining leaves the calendar idle rather than misconfigured.
- Hardened the generated sync wrapper for OAuth providers: only pairs able to run noninteractively are passed to vdirsyncer, an unauthorized Google pair is skipped with a log line instead of blocking a cron run on an interactive consent prompt, and discover/sync are bounded with `timeout` when available. Each eligible pair is synchronized independently, so one revoked or failed Google connection cannot block another Google or CalDAV calendar and cannot overwrite that failed pair’s prior dashboard mirror. Existing CalDAV pairs files remain valid without regeneration.
- Documented the Google Cloud preparation honestly, including enabling the CalDAV API, using a Desktop-app OAuth client, and publishing the consent screen to production so refresh tokens do not expire after seven days.

## [1.5.6-beta.3] — 2026-07-02

### Opt-in private CalDAV writeback

- Added local-first two-way calendar editing for exact private CalDAV/vdir collections explicitly enrolled during setup. Dashboard users can add an event from a day popup, edit a simple event without moving it between collections, and skip exactly one recurring occurrence; the local vdir is updated first and normal CalDAV sync is then queued in the background.
- Kept website and URL ICS subscriptions, broad multi-collection mirrors, unmanaged local files, Dash-Go-generated feeds, attendee/organizer events, and detached recurrence instances structurally read-only in both UI and server routes. Registered CalDAV mirrors can be hidden but are protected from Calendar Manager delete/trash operations, so Dashboard Control never deletes a remote calendar.
- Added safe direct-VEVENT editing that retains nested `VALARM` components and provider properties, rejects aggregate event files, preserves DTSTART timezone/date form for `EXDATE`, uses date-only all-day values with exclusive end dates, and requires an enabled Dashboard Control PIN before a one-time event may be deleted.
- Added calendar-writeback status and configuration controls plus exact-collection setup enrollment. Broad CalDAV discovery remains display-only; a provider whose local vdir does not materialize the selected exact collection stays read-only rather than risking an aggregate write.

## [1.5.6-beta.2] — 2026-07-02

### Frontend first-paint and calendar efficiency

- Changed runtime font assets from `no-store` delivery to revalidated delivery: the dynamic font stylesheet now carries a content ETag and font binaries use normal Last-Modified revalidation, avoiding unnecessary downloads and font parsing on unchanged kiosk relaunches.
- Coalesced tap-binding cleanup away from mutation bursts, deferred last-known-event snapshot persistence until after a visible calendar update is scheduled, and batched day-cell event fitting into calendar-wide write → read → write phases to avoid per-cell layout thrashing on low-power devices.
- Added an empty data favicon so kiosk launches no longer generate a `/favicon.ico` 404. Dashboard content, touch behavior, and profile defaults are unchanged.

## [1.5.6-beta.1] — 2026-07-02

### Reliability, repair, and long-lived requests

- Added bounded SIGTERM/SIGINT graceful HTTP shutdown so service restarts and updates can drain in-flight work rather than cutting it off abruptly.
- Corrected explicit GitHub Release resolution for repair when the installed `VERSION` is missing or damaged, and preserved the timestamp recorded when interrupted system-update state is recovered.
- Kept the global HTTP write limit for ordinary endpoints while extending only the bounded Microsoft To Do sync response and clearing it only for the long-lived To Do SSE stream; the stream now emits a lightweight heartbeat.
- Made atomic text and JSON writes flush file content, requested mode, and the parent-directory rename entry in durable order for removable-storage power-loss resilience.

### Compatibility, observability, and source hygiene

- Replaced the local PBKDF2 implementation with Go’s standard library while proving byte-identical legacy PIN derivation; retained the existing four-to-eight ASCII-digit PIN policy.
- Rendered safely escaped, rune-bounded map-fallback reasons; replaced stale release-numbered outbound User-Agent strings; and removed dead startup regular expressions and a committed runtime JSON artifact from the control-server source directory.
- Added focused regression coverage for graceful shutdown, To Do deadline/heartbeat behavior, repair resolution, PIN compatibility, durable replacement behavior, fallback-map output, stale update timestamps, removed regex references, and runtime-artifact exclusion.

## [1.5.5] — 2026-07-02

### Responsive rendering across landscape and portrait showcase resolutions

- **Calendar event chips:** on displays narrower than 1500 CSS pixels the event time now sits on its own line, so titles wrap at word boundaries instead of splitting mid-word beside a wide time chip, and titles hyphenate cleanly when a long word must break. Event and span-bar titles step down two pixels at these widths for better density.
- **Weekday header:** header cells no longer spill into neighboring columns, and portrait displays up to 1150 CSS pixels wide show abbreviated weekday names so all seven days stay visible on narrow columns (previously Friday and Saturday could be clipped on 1080×1920 portrait walls).
- **Clock:** the sidebar clock caps its size against the actual sidebar width, so the seconds and AM/PM suffix can no longer overflow off the panel edge on squeezed layouts; default layouts are pixel-identical.
- **Scroll edges:** the calendar week rows, agenda list, and 14-day weather list fade out at their pane edges instead of cutting text mid-glyph.
- **Sidebar text:** agenda titles and weather descriptions break only at word boundaries with hyphenation, replacing eager anywhere-breaks.

## [1.5.4] — 2026-07-01

### Dashboard Control and installation clarity

- Consolidated Dashboard Control’s competing styling layers, restored visible selected and pressed touch states in every theme, stabilized the six-tab rail, renamed the household/preferences tab to Settings, and made card accordion behavior consistent.
- Replaced accidental auto-fit layouts with count-aware touch grids: five choices balance as centered 3 + 2, six as 3 + 3, and Quick Actions keeps an intrinsic content height.
- Restored the advertised installer menu actions for Control PIN, dashboard service, and SSH; made Demo Mode non-destructive by default; corrected Control gesture and setup wording; and made menu routing, timeout retention, customization retry, and weather-provider selection self-consistent.

### Security, diagnostics, and repair resilience

- Hardened Control and personal-inbox PIN state machines with strict verifier semantics, fail-closed configuration reads, persistent escalating lockouts, server-enforced every-open expiry, safer credential changes, and cross-origin API rejection.
- Corrected Doctor repair-number selection and non-interactive fix safety, added privacy-preserving rotating-message fit diagnostics, and made repair backups external to the application tree with actionable verified-bundle recovery guidance for a damaged server binary.

### Weather and generated-calendar correctness

- Canonicalized daily precipitation to millimetres before source blending, retained the browser as the single robust blend authority, and added daily low/high coherence protection.
- Fixed DST-safe every-N-days chore cadence, ISS error-payload preservation, stale schedule-override resilience, bounded/collision-aware occurrence moves, duplicate month-end payday generation, holiday landing correction, truthful seasonal and leap-day observances, and moon-output reporting.

## [1.5.3] — 2026-07-01

### Calendar décor and visual clarity

- Added five static calendar decals for every seasonal, holiday, and calendar-aware observance theme, with richer user-selected décor density retained on every performance profile, including Lite.
- Refined Bold and High Contrast weather SVGs and modest crescent earthshine detail while preserving the established theme and calendar rendering model.

### Low-power rendering discipline

- Kept the new décor static and render-bound: no decoration polling, added network work, animation, external SVG assets, raster images, SVG filters, masks, or gradients.
- Decals are inserted only during a normal calendar render or an explicit visual or theme setting change, preserving the existing Lite memory and background-work posture.

## [1.5.2] — 2026-06-30

### Household experience

- Improved Lite message fitting and footer safety, refreshed the built-in household message catalog, and added calendar-aware observance wording that respects configured holiday sources.
- Added editable local Household Schedules for Payday, Trash Pickup, and Recycling Pickup, including safe one-time day-popup corrections for Dash-Go-owned occurrences.
- Made Dash-Go-owned Chore Wheel, Maintenance, and Routine completion controls reversible when the durable household record can safely return to its prior state.

### Themes and calendar clarity

- Curated the theme picker into purposeful groups, retired the More catchall, intentionally excluded Back to School and Game Day, and retained touch-safe four-to-six-column preview grids with Seasons fixed at four columns.
- Added event-backed Holidays & Observances themes, including gated Hanukkah and Kwanzaa availability, and improved calendar-legibility tokens across selected existing palettes.

### Reliability, privacy, and maintenance

- Hardened weather cache identity, backup selection, trusted calendar-link backup/restore, runtime font delivery, control previews, and fixed local fallbacks without adding background dashboard work.
- Preserved trusted calendar-link targets under the Dash-Go user home and `/Calendars`; unsafe paths, special files, and unsafe link chains fail before live restore replacement.
- Corrected backup ordering to compare full timestamps directly and extended focused source coverage around the new behavior.

### Documentation and release workflow

- Updated installation examples and current-release documentation for the public GitHub Releases workflow.
- Consolidated the 1.5.2 beta development record into this stable release entry.

## [1.5.1] — 2026-06-29

### Documentation and showcase assets

- Added repository-owned Dash-Go Showcase Studio screenshots for the dashboard, weather details, radar, Apps launcher, Family Message Board, Dashboard Control, themes, and future gallery use.
- Re-encoded every repository screenshot as a uniform `2034 × 1144` RGB PNG with no EXIF, XMP, ICC, text, timestamp, author, comment, GPS, or software metadata.
- Added a focused README screenshot gallery and linked the complete project screenshot set under `docs/screenshots`.
- Reduced the Raspberry Pi Imager visual walkthrough to one retained orientation image while preserving the full written headless-installation workflow.
- Preserved the 1.5.0 functional application, installer, updater, and UI baseline; only normal stable-release identity, browser-cache, and map user-agent references changed.

## [1.5.0] — 2026-06-29

### Product and interface

- Refined Dashboard Control, the shared on-screen keyboard, controls, buttons, and input styling for a more consistent touch workflow.
- Made the shared keyboard start with Shift active and added a context-aware affirmative key that completes the field’s existing action before closing the keyboard.
- Corrected light-theme input treatment and Family Message Board keyboard/scroll layering so normal form scrolling remains usable without a native scrollbar drawing over the keyboard.
- Kept Dashboard Control calm at opening, with cards collapsed until the user chooses a section.

### Reliability and household data

- Hardened calendar recurrence handling, including imported recurrence exceptions, timezone/DST behavior, recurrence-cache invalidation, and bounded generated feeds.
- Strengthened update coordination, stale-job recovery, rollback truthfulness, package-update locking, and kiosk return-to-dashboard behavior.
- Added tighter HTTP request/body limits and durable-write guards without changing normal loopback, PIN, or household-action behavior.
- Removed obsolete internal façades and verified that retained runtime paths remain purposeful.

### Architecture and maintainability

- Completed the 1.5 domain-boundary cleanup: semantic browser source names, manifest-owned browser order, and focused Go internal packages with narrower service boundaries.
- Preserved the local builder as the owner of generated browser assets, binaries, package validation, checksums, and GitHub Release asset preparation.
- Retired release-numbered test naming and the completed in-source architecture/refactor ledger in favor of domain/outcome test names and durable AI guidance.

### Setup and documentation

- Made fresh installations default to the stable release track.
- Rewrote the README around Raspberry Pi OS Lite and Raspberry Pi Imager, including an SSH-first visual setup path for a new headless Pi.
- Condensed current-state and release-history documentation around immediate operational truth and stable milestones.

## [1.4.4]

- Improved low-power dashboard rendering, calendar geometry, directional scroll overscan, idle return behavior, message fitting, and Dashboard Control layout stability.
- Deepened app lifecycle, touch, OSK, local-first data, and provider-integration safeguards across household tools.
- Strengthened package, installer, and browser source-structure checks for the local builder.

## [1.4.3]

- Added and matured household tools including People, Family Message Board inboxes, Chore Wheel, Maintenance, Routines, local To Do, Grocery, and optional Microsoft To Do synchronization.
- Improved update progress, backup/restore, Calendar Visibility, installer repair behavior, theme polish, diagnostics, and terminal access controls.
- Added optional Apprise-Go notifications with server-side secret handling and bounded delivery behavior.

## [1.4.2]

- Improved radar behavior, event/day overlays, Dashboard Control organization, display responsiveness, and kiosk resilience.
- Expanded Doctor/repair coverage and strengthened the transition to Go-owned runtime behavior.

## [1.4.1]

- Focused on stable operation: installer recovery, Doctor/repair clarity, autologin and kiosk recovery, health reporting, and low-memory appliance behavior.

## [1.4.0]

- Established the Go dashboard control server as the active runtime and release baseline.
- Preserved the kiosk-oriented browser experience while moving runtime control, update, diagnostics, and configuration behavior away from the retired Python service.

## Earlier releases

Earlier Dash-Go releases established the calendar/dashboard foundation, weather and message experience, theme system, local calendars, performance profiles, and touchscreen kiosk workflow.

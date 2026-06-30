# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release

- **Version:** `1.5.1`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, source archive, SPDX SBOM, and `SHA256SUMS`.
- **Release integrity:** published assets use immutable GitHub Releases; installation and update flows validate downloaded and staged content before managed files are replaced.

## Active development beta

- **Version:** `1.5.2-beta.5`
- **Track:** beta
- **Focus:** retain safe rotating-message fitting, calendar-aware observances, editable household schedules, and the curated 100-theme picker while making Dash-Go-owned day-popup completion checkboxes safely reversible.

`1.5.2-beta.5` clamps each message to the fitted line count, uses one bounded rendered verification before a new fit becomes trusted in cache, biases Lite Canvas width prediction conservatively, and reserves additional line-box headroom. The fixed bottom band can therefore ellipsize safely within its own bounds rather than letting a late wrapped line fall below the viewport.

The default catalog now uses calmer household-safe wording, preserves hidden/edited state when a built-in message is renamed, and recognizes holiday events only from loaded calendar sources. Installer-selected Jewish, Islamic, Christian, Orthodox Christian, and Hindu layers automatically enable their matching curated greetings when an exact celebration event is present. Neutral wording remains available for any enabled holiday calendar; solemn observances remain respectful; overlapping distinct occasions use inclusive wording instead of choosing one celebration.

`1.5.2-beta.5` adds a versioned local Household Schedules model for one or more named payday rules plus the existing Trash Pickup and Recycling Pickup feeds. Dashboard Control now edits recurring rules without rerunning the installer, while the day popup can move, skip, or restore one explicit Dash-Go-owned occurrence. Paydays support every-N-week, multiple monthly-date, and nth-weekday patterns with optional previous/next-business-day adjustment; each rule selects only installed holiday layers it should honor. Existing installer settings migrate once without changing their original schedule.

`1.5.2-beta.5` expands the theme catalog into curated Core, Readability, Color, Nature & Elements, Aesthetic, Fun, Materials, Practical, Seasons, Seasonal, and Holidays & Observances groups. The former More catchall is retired, Back to School and Game Day are intentionally absent, and the picker retains preview cards while choosing touch-safe 4–6-column grids per group. Hanukkah appears only when the enabled Jewish holiday layer contributes a recognized Hanukkah event today; Kwanzaa appears only when an enabled holiday-tagged source contributes Kwanzaa today. Event-backed observance themes remain local-cache-only and can take priority over the fixed seasonal-date helper when seasonal rotation is enabled.

`1.5.2-beta.5` makes the directly actionable day-popup checkboxes reversible for Dash-Go-owned Chore Wheel, Maintenance, and Routine events. Current and past chores can return from completed to assigned without touching assignment identity or fairness planning. Maintenance completion records preserve the original due/last-completed state and can be restored only while that completion remains the task’s latest safe state; later edits, reschedules, archives, restores, and newer completions keep the historic record visible but read-only. Routine checklist steps remain reversible after a routine is completed, while skipped and future sessions remain non-editable. All day-popup state comes from the server’s durable model and atomically refreshed app-owned calendar feeds.


`1.5.2-beta.5` also addresses the currently open CodeQL findings without adding background device work. Weather cache identity uses a keyed HMAC marker rather than a direct digest of the provider secret. Backup selection uses server-discovered regular-file records, while calendar symlink backups use structured trusted-root metadata that supports both the dashboard user’s home and the supported `/Calendars` root; unsafe paths, special files, symlinked backup ZIPs, and link chains that resolve outside those roots fail before live restore replacement. Runtime font requests select pinned metadata and serve opened, verified regular files; unchanged assets reuse a size/mtime verification cache. Dashboard Control previews use DOM APIs instead of interpolated style attributes, and test/bootstrap fallbacks now use fixed or parsed values. These are on-demand or static changes only: no extra dashboard startup work, network request, timer, or periodic filesystem scan is introduced. The corrected beta.5 source handoff also compares backup record `time.Time` values directly when ordering local archives, with a focused regression covering descending timestamp order.

## Recommended operating model

- **Fresh base:** Raspberry Pi OS Lite written with Raspberry Pi Imager, with hostname, normal user, localisation, network, and SSH configured before first boot.
- **Runtime:** the Go dashboard control server listens on loopback and Surf/WebKit displays the kiosk interface.
- **Primary reliability target:** Raspberry Pi Zero 2 W using the Lite profile.
- **Managed application tree:** `~/dashboard`.

## Product posture

- Dashboard Control opens calm with all cards collapsed.
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
- `CHANGELOG.md` records concise stable-release history and a brief active development section.
- `INTEGRATIONS.md` documents optional outside services and their local/offline behavior.
- `PRIVACY.md` documents local storage, optional network sharing, backups, and administrator responsibilities.
- `THIRD_PARTY_NOTICES.md` records distributed third-party software and asset notices.
- `AI.md` contains durable assistant guidance only and is intentionally excluded from source handoffs and release assets.
- The local Windows/WSL release builder remains authoritative for generated assets, compiled binaries, package validation, checksums, SPDX SBOM generation, and GitHub Release asset preparation.

This file is an immediate operating snapshot. Do not add beta journals, completed project history, detailed test logs, or prior-release issue lists here.

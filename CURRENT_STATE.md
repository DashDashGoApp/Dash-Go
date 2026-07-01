# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release

- **Version:** `1.5.3`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, source archive, SPDX SBOM, and `SHA256SUMS`.
- **Release integrity:** published assets use immutable GitHub Releases; installation and update flows validate downloaded and staged content before managed files are replaced.

## Current development beta

- **Version:** `1.5.4-beta.1`
- **Track:** beta
- **Focus:** Dashboard Control consolidation, PIN-security state-machine hardening, installer-flow repair, weather-data correctness, Doctor/repair safety, and generated-calendar resilience: visible selected/pressed touch states, one owned tab rail, safer PIN handling, a truthful non-destructive installer menu, canonical weather data, conservative repair recovery, DST-safe chore cadence, verified ISS preservation, and correctable generated schedule movement.

## 1.5.4-beta.1 focus

- **Dashboard Control clarity:** selected and pressed controls use visible theme-aware fills on every theme; the six-tab rail has symmetric internal spacing and stable two-column small-screen rows.
- **Control maintainability:** retired action-drawer, obsolete Control-page maintenance, five-tab, and unused grid layers were removed so layout, sizing, and state ownership are explicit.
- **Overview flow:** Device status opens by default and groups key network, device, and data-freshness signals, while lower-signal telemetry is available under More device details.
- **Touch workflow:** Settings is the clear household/preferences tab name, every page follows the same accordion model, and action feedback is shown beside the active action as well as in the existing global status line.
- **PIN security:** configured credentials are verified strictly rather than treating a disabled or unconfigured lock as a successful verification; Control and personal-inbox failures use bounded persistent escalating lockouts; every-open Control sessions use a short server-side expiry refreshed only while Control remains active; unavailable PIN configuration fails closed; browser-facing status payloads never include verifier material; browser API requests with a supplied cross-origin context are rejected.
- **Installer flow:** the Control PIN, dashboard service, and SSH menu actions are live and tested; the menu uses named identities with Exit last; Demo Mode defaults to keeping data; customization retries safely when review is rejected; current PIN-duration choices are preserved on Enter; pre-flight runs only after a real action is selected; and setup guidance names the moon-phase Control gesture and current Settings tab.
- **Weather integrity:** every active provider adapter converts daily precipitation to millimetres before returning source data; the Go server publishes normalized sources rather than a second divergent blend; the browser remains the authoritative robust blend and applies a final daily low≤high coherence guard.
- **Doctor and repair safety:** visible repair numbers resolve in the same safe→guided→admin order that Doctor renders; redirected `--fix` is safe-only unless explicit reviewed `--only` keys are supplied; repair archives live outside the application tree; and a missing/corrupt server repair failure prints a verified release-bundle recovery recipe.
- **Message fit observability:** Dashboard Control’s Rotating messages editor reports session-only final-safe-clip and rendered-correction counts without collecting message text or adding rotation-time layout work.
- **Generated calendar resilience:** every-N-days chores use civil-date math across DST; ISS refresh keeps the last good calendar on HTTP or provider-error payloads; stale schedule overrides are ignored safely at load; one-off moves stay in a visible ±90-day generation window and report a same-rule collision; calendar generation deduplicates clamped month-end paydays, avoids a second holiday landing, computes local astronomical season dates with Northern Hemisphere labels, preserves February 29 celebrations as February 28 observations in non-leap years, and reports moon output only when it succeeds.

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

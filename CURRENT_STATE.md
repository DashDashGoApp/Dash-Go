# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release

- **Version:** `1.5.4`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, source archive, SPDX SBOM, and `SHA256SUMS`.
- **Release integrity:** published assets use immutable GitHub Releases; installation and update flows validate downloaded and staged content before managed files are replaced.

## Current development beta

- No development beta is staged in this source handoff. `1.5.4` is the stable promotion candidate.

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

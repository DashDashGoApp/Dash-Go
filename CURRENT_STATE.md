# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release

- **Version:** `1.5.7`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Promotion status:** stable source handoff ready for local-builder validation and GitHub publication.
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, source archive, SPDX SBOM, and `SHA256SUMS`.
- **Release integrity:** published assets use immutable GitHub Releases; installation and update flows validate downloaded and staged content before managed files are replaced.

## Current beta source handoff

- **Version:** `1.5.8-beta.9`
- **Track:** beta
- **Minimum upgrade version:** `1.4.0`
- **Scope:** Installer fool-proofing and Trixie/X11 kiosk enforcement on top of the beta-series integrity work: bounded download and APT-lock retries, pre-commit storage checks, RAM/filesystem/time-zone preflight clarity, safe sudo/session concurrency handling, opt-in reboot clarity, and local black-screen rescue instructions. Dash-Go remains deliberately X11/LightDM/Openbox-based; Doctor now names an active Wayland or `greetd` mismatch rather than implying Wayland support.
- **beta.6 migration:** a verified beta.9 release bundle provides `./install.sh --bootstrap-selector-integrity` for a device still on beta.6. It atomically refreshes `~/install.sh`, verifies the copied installer, and launches the corrected beta updater so a drifted beta.6 selector is replaced from the manifest-owned payload. The full source gate includes an explicit beta.6 transaction rehearsal.
- **Validation boundary:** `app/tests/run-all.sh` is the required local/CI source gate: it runs formatting, Go module/vet/test checks, shellcheck, all shell smokes, and all Node/browser smokes. The local Windows/WSL builder remains responsible for generated assets, compiled binaries, package validation, checksums, SBOM, and publishable GitHub Release assets.

## 1.5.7 stable scope

- **Recurrence safety:** series time edits now preserve EXDATE occurrence identity when a UTC-form DTSTART crosses UTC midnight. Explicit TZID/DST and floating-time coverage protects the corresponding local-time paths.
- **Final-delete synchronization safety:** the one-run empty-collection override is armed only after a verified final local-event delete. Its exact source, pair, and collection authorization is persisted for one bounded automatic retry across a failed targeted run or server restart, revalidated against the still-empty collection before use, and cleared by a later non-final mutation or successful sync.
- **Truthful sync status:** a zero-exit provider authorization skip or unrecognized wrapper result is shown as attention rather than a false synchronized result.
- **Deliberate scope boundary:** legacy pair-less queue coalescing, update-version recovery hardening, version-grammar alignment, and update-preflight I/O caching remain follow-up review items rather than unverified changes in this focused release.

## 1.5.6 stable highlights

- **Private calendars:** Google, iCloud, and compatible CalDAV connections now use a review-only discovery step followed by explicit exact collection selection. Selected collections are independently display-only or editable, sync serially and at low priority, and retain their own safe status and recovery state.
- **Two-way event management:** eligible exact private calendars support local-first event creation, editing, deletion, one-occurrence recurrence changes, and constrained simple-series updates. Unsupported provider-managed, attendee, invitation, advanced recurrence, broad-mirror, URL-feed, and generated-calendar cases remain clear read-only flows.
- **Conflict and recovery safety:** conflicts stop safely, appear in Calendar Manager, and require a PIN-gated one-shot deliberate winner choice for the affected pair. Repair connection performs one targeted discovery/sync for an exact selected pair without broad account changes. Normal configuration never stores a permanent winner.
- **Update and installer safety:** normal SSH and Dashboard Control updates are strict upgrades only, with successful no-op behavior for equal releases and no downgrade path. Private-calendar setup reports the actual provider/package operation that fails rather than using an unrelated weather probe as an internet test.
- **Calendar control and visual polish:** Calendar Manager has one continuous scroll surface; event capabilities refresh after edit-state changes; recurring event management is scoped and clear; event forms use Dash-Go-owned calendar selection, touch-safe quick time controls, semantic states, focused keyboard treatment, and theme-consistent recovery/error surfaces.
- **Pi appliance behavior:** calendar work remains bounded, serial, discovery-free during routine sync, and low priority. The dashboard preserves last-known calendar data through provider failures and avoids an always-on vdirsyncer process.

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

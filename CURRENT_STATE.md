# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current beta release candidate

- **Version:** `1.5.10-beta.2`
- **Track:** beta
- **Purpose:** Calendar and Weather reliability beta based on 1.5.10-beta.1, preserving the dashboard layout and interaction model while correcting provider units, hardening forecast blending, and reducing redundant Calendar work.
- **Primary changes:** canonical millimetre precipitation with display-edge conversion, robust freshness-aware Weather blending, per-provider cache and request telemetry, typed rate-limit handling, settings-aware Calendar cache windows, source-digest reuse, serialized cache generation, ETag/304 browser reuse, stable event identities, and corrected ICS cancellation/revision/duration handling.
- **Required candidate proof:** Dash-Go Local Builder 1.0.42 or newer must use Go 1.26.5, prove `go mod tidy -diff` and `go mod verify`, regenerate and verify minified JavaScript and CSS bundles, run the complete source and Chromium layout gates, build all five Linux targets reproducibly, derive the binary-linked SPDX inventory, and pass installer, package, checksum, and final-archive validation before publication.
- **Compatibility:** existing settings, calendar source files, and provider configuration remain compatible. Calendar cache schema 10 rebuilds automatically; Weather precipitation remains canonical millimetres internally and is converted only for display.

## Current stable release

- **Version:** `1.5.9`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Promotion basis:** 1.5.9 consolidates the validated 1.5.9 beta line into one stable source contract. The local Windows/WSL builder remains responsible for the final generated bundles, five Linux binaries, package validation, binary-derived SPDX SBOM, checksums, release archives, and publishable GitHub assets.
- **Required release proof:** Dash-Go Local Builder 1.0.42 or newer must use Go 1.26.5, prove `go mod tidy -diff` and `go mod verify`, regenerate and verify minified browser bundles, clear the complete source and Chromium layout gates, derive one matching authenticated module inventory from all five released Linux binaries, and pass installer, package, checksum, SBOM, and final-archive validation. Dash-Go GitHub Publisher 1.4.11 or newer must extract, preview, publish, and verify the exact structured 1.5.9 changelog section.
- **Compatibility:** existing event-cache metadata is accepted and upgraded after one safe fallback parse. `events.cache.json` and normal browser/API content remain compatible. The service memory setting is a soft Go runtime target rather than a systemd hard limit.
- **Source-handoff contract:** `app/tests/run-all.sh`, `.github/workflows/verify.yml`, and `RELEASING.md` travel together. `AI.md`, `.git/`, generated bundles, binaries, builder tooling, release artifacts, mutable user data, and credentials remain excluded.
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and immutable GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, public source archive, SPDX SBOM, and `SHA256SUMS`.

## 1.5.9 stable scope

- **Native Showcase contract:** `dashgo-showcase/v1` supplies isolated scenario data, manifest-owned calendars, bounded cache preparation, readiness reporting, package probing, Windows portability, and strict static-data routing while remaining dormant during ordinary Dash-Go operation.
- **Debian security maintenance:** valid Debian mirrors, proxies, and mirrored archives qualify through current Debian-signed base and `trixie-security` metadata rather than hostname matching. Insecure, wrong-suite, missing-metadata, and incompatible source layouts remain fail-closed; repair is explicit and rollback-safe.
- **Calendar and weather presentation:** fixed local-calendar labels render correctly in edit flows, and refreshed static calendar décor and weather SVG sets preserve the existing low-power inline-asset model.
- **Pi performance and durability:** generated JavaScript is minified without binding renames, unchanged event caches use validated metadata instead of full reparsing, compact cache writes are atomic, redundant cache URL churn is removed, and the dashboard service receives a 96 MiB soft Go memory target.
- **Release assurance:** the source runner, CI workflow, and maintainer instructions travel together; ShellCheck and module-tidy policy are explicit; native Showcase declarations are cross-compiled and probed; SPDX reports modules linked into all five shipped binaries; and GitHub notes are generated from the reviewed stable changelog section.

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

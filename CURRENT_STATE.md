# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release candidate

- **Version:** `1.5.10`
- **Track:** stable
- **Promotion basis:** the complete validated `1.5.10-beta.5` source is promoted without runtime feature changes. Stable promotion synchronizes release identity, public documentation, cache busters, the source-feature contract, and one consolidated stable changelog section.
- **Primary scope:** Calendar and Weather correctness and caching; same-origin and outbound-network hardening; truthful last-known data; success-based display sleep and wake reconciliation; scoped frontend invalidation; typed JSON/JSON v2 readiness; Pi Zero 2 W background-work reductions; and source-lineage/repository-handoff assurance.
- **Required release proof:** Dash-Go Local Builder 1.0.42 or newer must use Go 1.26.5, extract and validate this exact repository-ready stable source handoff, prove the 24-feature source contract and focused JSON v2 lane, run `go mod tidy -diff`, `go mod verify`, vet, tests, Linux race coverage, all browser and Chromium layout gates, regenerate and verify minified JavaScript and CSS bundles, build all five Linux targets reproducibly, derive the binary-linked SPDX inventory, and pass installer, package, checksum, catalog, and final-archive validation before publication.
- **Physical stable proof:** on the Pi kiosk, verify scheduled screen-off, failed/offline screen-off recovery, touch wake, manual wake, scheduled wake, disabling the schedule while asleep, one post-wake reconciliation, Weather refresh and AQI behavior, Calendar last-known freshness, and normal Dashboard Control interaction.
- **Compatibility:** existing settings, calendars, private-calendar mappings, provider configuration, local apps, display schedules, four-digit PINs, and the repository-ready source workflow remain compatible. No user-visible feature or persistent data is removed.
- **Current release boundary:** this is a stable source handoff. It is not a deployable or published release until the local Windows/WSL Builder completes the pinned-toolchain, generated-asset, browser, package, architecture, catalog, checksum, SBOM, and final-ZIP gates.

## Current published stable release

- **Version:** `1.5.9`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Status:** remains the published stable release until the validated 1.5.10 Builder output is published and verified.
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and immutable GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, public source archive, SPDX SBOM, and `SHA256SUMS`.

## 1.5.10 stable scope

- **Calendar correctness:** truthful schema-2 last-known snapshots, broader ICS parity, stable occurrence identity, source diagnostics, conditional reads, bounded rebuild serialization, and periodic source rehash protection.
- **Weather reliability:** same-origin browser access, nonblocking cached AQI, partial and parallel provider work, freshness-aware blending, canonical-unit caches, stable location caches, and corrected precipitation/wind conversions.
- **Security:** loopback Host/Origin enforcement, narrowed CSP, rooted static and map serving, post-validation operation limiting, stricter POST fetch metadata, provider-data DOM hardening, and comprehensive custom-endpoint address policy including mapped and translation forms.
- **Pi performance:** scoped settings invalidation, sleep-aware minute work, stable no-store URLs, bounded diagnostics, provider/cache reuse, and serialized heavy refresh paths.
- **Release assurance:** a 24-feature lineage contract, JSON v2 lane, typed closed boundaries, Go/browser Calendar parity corpus, behavior-backed security and display-sleep tests, and one repository-ready handoff contract.

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

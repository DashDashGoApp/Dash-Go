# Dash-Go — Current State

**Dash-Go** is pronounced **“Dash Dash Go.”**

## Current stable release candidate

- **Version:** `1.5.15`
- **Track:** stable
- **Promotion basis:** a validated `1.5.15` source handoff. Stable promotion synchronizes release identity, public documentation, cache busters, the source-feature contract, and one consolidated stable changelog section.
- **Primary scope:** remove work, reclaim disk, and speed up maps — parallel tile fetches with one shared transport and a bounded render deadline, throttled map-cache cleanup, an in-memory geocode memo, ETag revalidation for cached map images, prewarm that renders the configured style first, a no-behaviour-change split of the HTTP static file, a regression test proving a shutdown grace expiry exits cleanly, quieter interrupted-update recovery logging, and housekeeping that prunes stale update-rollback snapshots (keeping the newest and its metadata) plus old doctor-set-aside weather caches.
- **Required release proof:** Dash-Go Local Builder must use Go 1.26.8, extract and validate this exact repository-ready stable source handoff, prove the 43-feature source contract and focused JSON v2 lane, run `go mod tidy -diff`, `go mod verify`, vet, tests, Linux race coverage, all browser and Chromium layout gates, regenerate and verify minified JavaScript and CSS bundles, build all five Linux targets reproducibly, derive the binary-linked SPDX inventory, and pass installer, package, checksum, catalog, and final-archive validation before publication.
- **Physical stable proof:** on the Pi kiosk, verify the four boring numbers (server RSS, WebKit RSS, zram used, server CPU time) show no regression, that a cold hybrid map render is at least 40 % faster than the 1.5.14 baseline, that `cache/` drops below 60 MB after the update with rollback still working, and that the update path installs cleanly with a clean server stop.
- **Compatibility:** existing settings, calendars, private-calendar mappings, provider configuration and keys, local apps, display schedules, four-digit PINs, and the repository-ready source workflow remain compatible. No user-visible feature or persistent data is removed.
- **Current release boundary:** this is a stable source handoff. It is not a deployable or published release until the local Windows/WSL Builder completes the pinned-toolchain, generated-asset, browser, package, architecture, catalog, checksum, SBOM, and final-ZIP gates.

## Current published stable release

- **Version:** `1.5.14`
- **Track:** stable
- **Minimum upgrade version:** `1.4.0`
- **Status:** remains the published stable release until the validated 1.5.15 Builder output is published and verified.
- **Official distribution model:** the [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and immutable GitHub Releases.
- **Release asset contract:** each published release provides a versioned installation bundle, public source archive, SPDX SBOM, and `SHA256SUMS`.

## 1.5.15 stable scope

- **Map speed:** tiles are fetched in parallel (capped at three in flight) over one shared connection pool with warm TLS sessions, so a hybrid preview no longer pays a fresh handshake per tile; each render attempt is bounded by a total deadline instead of stacked per-tile timeouts; the first imagery failure aborts the remaining fetches; and draw order is preserved.
- **Map cache efficiency:** the full image/tile cleanup directory scans now run at most once every twelve hours instead of after every render; successful geocode results are memoized in memory (invalidated by any on-disk cache change) so repeated popups skip the 33 KB cache re-read; cached map images answer `If-None-Match` with a 304 instead of re-downloading up to ~400 KB; and prewarm renders the configured style and its zooms before the rarer variants.
- **Lifecycle integrity:** a regression test now proves a shutdown whose grace period expires still exits cleanly (the shape behind the historical `context deadline exceeded` unit restart), and interrupted-update recovery logging no longer reports a benign missing action-history entry as a fault.
- **Disk reclamation:** housekeeping prunes update-rollback payload trees older than fourteen days — keeping the newest failed snapshot of each kind and its tiny metadata evidence — and caps doctor-set-aside weather caches to the newest one. A smoke test proves a fresh pending snapshot (a running update) is never touched.
- **Source structure:** the HTTP static file was split from the server lifecycle file with no behaviour change, keeping both well inside the navigability guards.

## 1.5.14 stable scope

- **Weather correctness:** a guarded feels-like metric so a provider that omits apparent temperature cannot render a false `0°`; a corrected Visual Crossing hourly clock (time-only entries joined to their parent day) that previously dropped every row while reporting success, now covered by mapper and regression fixtures; an air-quality failure cooldown that keeps the last good reading and stops doomed retries; and credential redaction, so a stored or displayed provider error cannot carry an API key.
- **Calendar and popup responsiveness:** gesture-aware refresh-time scroll restoration in both panes (a rebuild that lands mid-gesture no longer snaps the agenda to the top or writes over the viewer on the calendar); live re-render of an open day or event popup when calendar data is committed; bounded List-view staging in the Lite day popup; app-owned household popups that paint the calendar's own rows while their projection is in flight and cancel reads but never a completion; and Lite popup latency marks measured against a stated shell/content budget.
- **Pi performance:** the Lite calendar's row-culling and list-overscan controllers re-arm their settle timers inside the animation frame they already coalesce, so a scroll burst costs no timer churn, with a fast-flick fixture proving the prewarm window rather than assuming it.
- **Source structure:** the weather and calendar-grid sources were split into focused modules with no behaviour change, with the bundle manifest, asset-manifest test, and every dependent smoke updated to follow the moved code; the weather review surface now covers current-condition disagreement and hourly provenance.

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

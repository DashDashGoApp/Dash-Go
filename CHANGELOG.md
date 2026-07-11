# Dash-Go Changelog

## [1.5.10-beta.2] — 2026-07-11

Dash-Go 1.5.10-beta.2 is a Calendar and Weather reliability beta. It keeps the existing dashboard layout and interaction model while standardizing provider units, hardening multi-source forecast blending, reducing unchanged Calendar work, and improving recurrence correctness and source diagnostics.

### Improvements

- Standardized Weather precipitation totals on canonical millimetres at every provider boundary, converting only at the display edge for metric or imperial presentation.
- Added source-aware Weather detail rows with precipitation totals, cache age, request duration, request count, response size, and HTTP status where available.
- Made multi-source Weather blending freshness-aware and field-specific: three-source disagreements use robust medians, precipitation totals and probabilities resist one-source extremes, stale values are excluded while fresh values exist, and missing condition codes no longer vote as clear weather.
- Replaced numeric weather-code tie bias with condition-family voting and supporting precipitation signals, and selected sunrise and sunset from the median-nearest provider timestamp.
- Gave each Weather provider an independent quota-safe freshness interval while retaining the normal profile-based aggregate check cadence.
- Made manual Weather refresh attempt live provider requests while continuing to respect active authentication and rate-limit cooldowns and retaining stale fallback data on failure.
- Made the Calendar event-cache window follow configured Calendar and Agenda coverage, including the selected first day of the week and a bounded safety margin.
- Added conditional event-cache loading through ETag/304 so unchanged browser refreshes reuse the parsed event model instead of reparsing the full cache.
- Reused persisted Calendar source digests when source path, size, and nanosecond modification time are unchanged.

### Bug fixes

- Corrected imperial precipitation totals that previously displayed canonical millimetres with an inch symbol.
- Corrected Visual Crossing wind conversion when metric provider responses report kilometres per hour.
- Included WeatherAPI snow probability when determining a day’s maximum precipitation probability.
- Replaced Pirate Weather mixed physical accumulation with hourly liquid-water-equivalent precipitation totals grouped by forecast-local day.
- Corrected Calendar change detection so same-length edits to event descriptions or locations invalidate the browser event model.
- Made generated occurrence identifiers stable across unrelated earlier-event additions or removals.
- Added ICS `STATUS:CANCELLED`, revision selection through `SEQUENCE` and revision timestamps, cancelled recurrence overrides, and `DURATION` support in both the Go cache parser and emergency browser fallback.

### Performance and reliability

- Serialized in-process Calendar cache generation and added a bounded cross-process lock so cron, sync, writeback, and household updates do not rebuild the same cache concurrently.
- Serialized in-process aggregate Weather refreshes so simultaneous dashboard, manual, and command-line requests do not duplicate provider calls.
- Advanced the Calendar cache schema and fingerprint contract so corrected recurrence and identity behavior rebuilds existing caches safely.
- Added typed Weather HTTP errors with `Retry-After` handling and serialized provider cooldown-state updates to prevent concurrent read-modify-write loss.
- Added per-provider live-request telemetry without introducing extra provider calls.
- Made all-excluded Weather source recovery explicit and retained clear metadata when stale provider data is used only as a last-known fallback.

### Build and release integrity

- Added Calendar and Weather contract coverage for provider units, robust blend behavior, stale-source selection, request telemetry, cache-window sizing, digest reuse, conditional cache reads, stable event identity, and ICS cancellation/revision/duration semantics.
- Retained the focused JSON v2 compatibility lane and existing source, browser, race, five-architecture, generated-asset, package, SBOM, checksum, and final-archive gates.

### Upgrade notes

- Existing Calendar event caches are rebuilt automatically for cache schema 10; calendar source files, settings, and browser-visible event content remain compatible.
- Weather provider caches remain provider-specific and are refreshed as each source becomes due; changing display units continues to preserve the configured provider set and blend behavior.

## [1.5.10-beta.1] — 2026-07-10

Dash-Go 1.5.10-beta.1 is a behavior-preserving cleanup beta based on 1.5.9 stable. It keeps the same end-user features, displayed content, interaction semantics, refresh cadence, API response content, and persistent file formats while reducing duplicated frontend infrastructure and small amounts of unnecessary browser and server work.

### Improvements

- Consolidated all lazy household-app stylesheet and script loading onto one retry-safe asset lifecycle while preserving each app's existing load order, failure cleanup, and retry behavior.
- Centralized Lite and Pi Zero profile recognition so Calendar, messages, radar, Dashboard Control, and responsive layout use one canonical profile classification.
- Centralized two-tap destructive-action arming and timer cleanup while preserving each control's existing label, confirmation window, and action behavior.
- Shared the regular and interactive-map on-screen keyboard character rows so both touch keyboards retain identical keys without duplicate definitions.
- Replaced Dashboard Control's serialized JSON equality check with member-order-independent JSON-value comparison in preparation for Go JSON v2 behavior.
- Moved major Dashboard Control visibility state from inline `display` mutations to semantic `hidden` state while preserving the same computed layout and focus lifecycle.

### Performance and reliability

- Removed redundant timestamp query parameters from local no-store settings, calendar, weather, ICS, and message requests, keeping stable resource identities without changing freshness.
- Added deterministic minification for builder-generated dashboard and Dashboard Control CSS while retaining release identity headers and readable split CSS sources.
- Removed a per-request static alias-map allocation from the Go HTTP path by using the same fixed alias contract through a switch.

### Build and release integrity

- Added focused source coverage for stable no-store URLs, CSS minification, shared lazy loading, canonical profile classification, confirmation cleanup, shared keyboard rows, semantic JSON equality, hidden-state shells, and the allocation-free static alias path.
- Kept generated browser assets builder-owned and excluded from the source handoff; the local builder must regenerate and verify both minified JavaScript and minified CSS before publication.

## [1.5.9] — 2026-07-10

Dash-Go 1.5.9 expands the native Showcase contract, improves Debian security-maintenance recognition, refreshes calendar and weather artwork, and reduces background work on the Pi Zero 2 W while strengthening the source, builder, SBOM, and GitHub release workflow.

### Features

- Added the dormant-by-default native `dashgo-showcase/v1` contract for Dash-Go Showcase Studio, with an immutable application tree, isolated disposable scenario data, manifest-owned calendars, bounded cache preparation, an authoritative readiness endpoint, and a package-contract probe. Ordinary Dash-Go installations remain outside Showcase mode.

### Improvements

- Made the Showcase contract portable to Windows by isolating platform-specific locking, process, signal, directory-durability, disk-reporting, and detached-process behavior behind native helpers while preserving Linux appliance behavior.
- Expanded managed Debian security-maintenance eligibility to authenticated country mirrors, caching proxies, and mirrored local archives when their enabled base and `trixie-security` metadata is current and Debian-signed. Explicit repair remains opt-in and rolls back Dash-Go-owned source changes on failure.
- Refreshed seasonal, holiday, and observance calendar artwork together with every weather icon style and weather-summary metric icon while preserving static inline-SVG, five-artwork, and Lite-profile constraints.

### Bug fixes

- Corrected Showcase static-data routing on Windows by canonicalizing URL paths before applying the isolated-data allowlist and filesystem conversion, while continuing to reject drive-qualified and ambiguous paths.
- Corrected fixed local-calendar labels in event-edit states so calendar names and providers render as structured elements rather than stringified markup.

### Performance and reliability

- Minified builder-generated JavaScript without renaming classic-script bindings, reducing browser source volume while preserving cross-file identifiers.
- Reduced unchanged Calendar cache work by persisting validated summary and file-stat metadata, safely upgrading older metadata after one fallback parse, and avoiding a complete event-cache reparse when the source fingerprint and cache file are unchanged.
- Moved compact event-cache output onto the durable atomic-write path and removed the redundant timestamp query from browser cache requests.
- Added a `GOMEMLIMIT=96MiB` soft runtime target to both dashboard-service generation paths to keep Go memory growth bounded on low-memory appliances without imposing a hard systemd cap.
- Normalized floating-point artifacts in generated weather SVG coordinates without changing their rendered geometry.

### Security

- Hardened Debian source qualification by refusing insecure, trusted-without-verification, wrong-suite, missing-metadata, and incompatible Raspbian layouts with explicit Doctor reasons.
- Preserved strict Showcase static-data isolation and no-store behavior while adding URL-path canonicalization and regression coverage for Windows-shaped separators and native scenario delivery.

### Build and release integrity

- Added the Contract v1 Windows cross-compile and built-binary release probes so a source handoff cannot publish an undeclared, unbuildable, or misidentified native Showcase runtime.
- Made the public source handoff carry its full local/CI gate, GitHub verification workflow, and maintainer release instructions together.
- Standardized ShellCheck 0.9.0 policy, added an exact `go mod tidy -diff` gate, and retained fail-closed formatting, module, vet, test, race, browser, installer, package, checksum, and archive validation.
- Improved SPDX accuracy by deriving the Go runtime dependency inventory from authenticated module metadata embedded in all five released Linux binaries rather than treating graph-only or upstream test modules as shipped runtime dependencies.
- Added structured GitHub release notes sourced from the exact current-version changelog section, including canonical optional categories, pre-draft preview, source-asset agreement, and initial GitHub-body verification.

### Upgrade notes

- Existing event-cache metadata is accepted and upgraded automatically after one safe fallback parse; the browser-facing `events.cache.json` format remains unchanged.
- The dashboard service now uses a 96 MiB soft Go memory target. It is not a hard memory cap, and normal systemd restart behavior remains unchanged.

## [1.5.8] — 2026-07-04

### Calendar access and guided private accounts

- Clarified the calendar setup boundary: read-only iCalendar links remain view-only, while owned Google, iCloud, and compatible CalDAV calendars can use explicitly selected two-way synchronization.
- Added account-first private calendar setup with discovered calendar names, per-calendar read-only or editable state, first-sync-before-activation, and provider-aware migration guidance.
- Added Go-native Google OAuth for both computer-browser loopback and kiosk phone/tablet paste-back flows, plus guided iCloud and compatible CalDAV setup with owner-only credentials and draft cleanup.
- Hardened the vdirsyncer tool ladder with supported package, pipx, isolated Dash-Go virtual-environment, and explicitly approved user-level fallback paths; routine calendar work remains bounded and low priority.

### Update integrity, maintenance, and diagnostics

- Added managed Debian security maintenance for supported Debian and compatible 64-bit Raspberry Pi OS systems: security-only unattended upgrades, dormant backports pinning, bounded maintenance timers, and Doctor repair coverage without general feature upgrades.
- Corrected Doctor weather-cache location validation and kiosk duplicate detection so normal Pi launcher trees and healthy current cache schemas are not misreported as faults.
- Made payload replacement manifest-safe: package-owned selectors are preserved, post-commit and rollback trees are verified, and missing legacy selectors can be reconstructed for every shipped architecture.
- Added a one-time beta.6 selector-integrity bridge. A device still on `1.5.8-beta.6` can refresh its updater from the verified 1.5.8 bundle before its first ordinary stable update, so selector drift is repaired from the manifest-owned payload.
- Hardened installer-owned configuration editing: multiline `config.local.js` values are replaced safely, field names are treated literally, explicit JSON/string modes avoid accidental type coercion, and malformed JSON paths refuse silent data loss.

### Installer and kiosk resilience

- Added bounded retry behavior for verified release downloads and APT work, including clear package-lock guidance while preserving Dashboard Control’s narrow sudo permissions.
- Added pre-commit update storage checks, home/dashboard write probes, Lite-aware RAM guidance, and an explicit validated time-zone prompt for UTC or unset systems; Dash-Go never guesses a time zone silently.
- Prevented duplicate interactive installer runs, kept an approved sudo session alive through longer system work, and added clear reboot guidance after boot-affecting changes.
- Hardened the deliberate X11/LightDM/Openbox kiosk policy for Trixie-era systems by handling `greetd`, reporting active Wayland mismatches, and verifying LightDM’s configured/enabled state. Wayland kiosk support is not introduced.
- Added a concise local black/frozen-screen rescue route in the installer and README.

### Release assurance

- Added a required local/CI source gate covering Go module/vet/test/build work, formatting, ShellCheck, shell smokes, and Node/browser smokes, with a GitHub Actions verification workflow.
- Added regression coverage for selector recovery on all shipped architectures, beta.6 migration, update rollback, OAuth route behavior, Debian platform fallback, installer guardrails, and release identity/cache-buster consistency.

## [1.5.7] — 2026-07-03

### Calendar writeback safety and recurrence correctness

- Corrected recurring-series EXDATE remapping when a UTC-form DTSTART time edit crosses UTC midnight, preserving the excluded occurrence rather than allowing it to reappear. Explicit TZID/DST and floating-time recurrence coverage remains in place.
- Restricted vdirsyncer’s one-run empty-collection override to a verified final local-event delete for the exact selected source, pair, and collection. Ordinary deletes continue to use the normal empty-collection guard.
- Persisted that verified final-delete authorization for one bounded automatic retry across a failed targeted run or server restart. Every retry rechecks that the registered collection remains empty, and later non-final mutations or successful synchronization clear the authorization.
- Corrected writeback status reporting so zero-exit authorization skips and unrecognized wrapper outcomes are shown as attention rather than falsely reported as synchronized.

### Release documentation

- Updated the stable release contract, runtime cache-buster references, installation example, integration documentation, current-state record, and this changelog for 1.5.7.

## [1.5.6] — 2026-07-03

### Private calendar management and safe two-way synchronization

- Added user-led discovery and explicit per-collection activation for Google, iCloud, and compatible CalDAV calendars. Discovery remains review-only; every selected collection receives an exact local vdir mapping and an independently managed Dashboard source. Broad legacy mirrors remain read-only and preserved.
- Added local-first create, edit, delete, one-occurrence recurrence changes, and constrained simple-series management for eligible exact private calendars. Unsupported advanced recurrence, attendee/organizer, invitation, URL-feed, generated, and ambiguous provider-managed events stay explicitly read-only with clear guidance.
- Added targeted per-calendar vdirsyncer work, serial coalescing, low-priority execution, per-source status, and durable cron outcome reporting. A routine sync never performs broad discovery and never stores a permanent automatic conflict winner.
- Added Calendar Manager recovery for exact selected sources: targeted repair discovery, provider-specific authorization guidance, and PIN-gated one-shot conflict resolution that deliberately retains either the remote or Dashboard side for one affected calendar pair. Resolver configuration is temporary and owner-only; a bounded local snapshot precedes a remote-winner run.
- Hardened recurrence and vdir behavior: direct EXDATE values are preserved/remapped for supported series changes, detached overrides are matched safely, skip refuses an already-overridden occurrence, new overrides inherit the master sequence, duplicate UIDs remain fail-closed, and one Dashboard-initiated final-event delete may cross vdirsyncer’s empty-local guard for that single targeted run only.

### Update safety, installer clarity, and kiosk resilience

- Normal SSH and Dashboard Control updates now require a strictly newer compatible release. Equal versions are successful no-ops, lower versions do not downgrade a device, and repair remains the explicit same-version recovery path.
- Removed the unrelated weather-provider internet probe from private-calendar setup. Network failures are now diagnosed at the actual APT, pipx, Google OAuth/CalDAV, iCloud, or selected CalDAV operation with bounded, secret-safe wording.
- Retained Go-owned shutdown, durable file replacement, bounded background work, exact architecture staging, and Pi Zero 2 W-oriented low-memory safeguards across installer, service, kiosk, and calendar paths.

### Calendar experience and visual consistency

- Reworked Calendar Manager into one page-owned scroll surface, added clear healthy/neutral/warning/conflict semantics, and made selected-calendar editing state and live event capability updates consistent.
- Added Manage event and Manage recurring event flows, quick start and length controls, inclusive all-day editing, and theme-consistent conflict/recovery guidance.
- Replaced browser-native event-calendar selection with Dash-Go-owned touch-safe ownership and picker surfaces. Improved state hierarchy, focus treatment, pressed states, quick-time grouping, popup error emphasis, and theme-safe styling without WebKit-dependent color functions.

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

# Dash-Go Changelog

This changelog records stable Dash-Go milestones. Detailed development increments are consolidated at stable promotion so the file remains useful as a product history rather than a release-by-release development journal.

## Unreleased — 1.5.0-beta.38

- Public-transition privacy correction: public source tests no longer contain private historical markers; the local builder performs the exact private audit outside the repository.
- Clarified the public GitHub Release transition and conditional private vulnerability-reporting channel.

- Retains the maintainer release guide for privacy-reviewed source import, exact Git tag, local release build, draft-release review, asset verification, and public transition.
- Clarified that private GitHub releases can be rehearsed by authorized maintainers without putting a GitHub token on a Dash-Go VM or Pi; the installed updater remains deliberately anonymous.
- Made the security-reporting policy conditional on the actual GitHub repository setting; enable and verify private vulnerability reporting before public prerelease publication.
- Kept normal installed-device updates, Doctor online validation, and repair resolution on the canonical GitHub Release transaction with the beta.35 integrity/recovery guarantees intact.

## Earlier migration increment — 1.5.0-beta.36

- Added a local release-bundle identity path for private/offline device rehearsal. An extracted bundle exposes a side-effect-free `--bundle-info` command and forces its own beta/stable track so a beta archive cannot accidentally write a stable device update preference.
- Added matching local-builder device tooling: a non-destructive SSH smoke for the four GitHub Release assets and an explicitly confirmed SSH staging/install helper for a VM or Pi. Both verify `SHA256SUMS`; neither adds a GitHub token or an alternate runtime update source to the device.

## Earlier migration increment — 1.5.0-beta.35

- Added bounded private ETag caching for canonical GitHub Release metadata: 304 responses reuse validated cached metadata, and a GitHub rate-limit response may reuse metadata for no more than six hours without downloading or installing anything.
- Removed checksum-bypass installer wording and the remaining executable references to retired distribution metadata; release resolution remains GitHub-only, exact-asset, digest-checked, checksum-checked, staged, and rollback-safe.
- Added source and builder contracts that distinguish the private transitional managed baseline from the public GitHub Release asset set and block reintroduction of retired updater vocabulary into executable code.

## Earlier migration increment — 1.5.0-beta.34

- Moved the active installer, updater, repair path, and Doctor online validation to the compiled canonical GitHub Release resolver.
- Added fail-closed GitHub asset handling: exact immutable release metadata, GitHub SHA-256 digests, `SHA256SUMS`, private staging, selected-architecture verification, generated-asset validation, and atomic managed-file replacement.
- Made the GitHub Release bundle self-contained for fresh installation and one-time migration from an pre-GitHub device; it carries both `install.sh` and the verified app payload.
- Changed plain `--repair` to restore the exact installed release; `--repair --update` is the explicit newest-release recovery path.
- Replaced historical update-host/profile handling with owner-only Stable/Beta track state and removed arbitrary release endpoint or credential use from the executable updater path.
- Updated the local builder’s selected output to a validated GitHub Release asset directory while preserving its internal full-webroot baseline only for transitional managed-baseline continuity.
- Added source and builder contracts for GitHub release progress, migration-state scrubbing, repair targeting, and atomic public asset-directory publication.

## [1.5.0] — 2026-06-28

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

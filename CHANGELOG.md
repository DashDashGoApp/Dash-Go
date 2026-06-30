# Dash-Go Changelog

This changelog records stable Dash-Go milestones. Detailed development increments are consolidated at stable promotion so the file remains useful as a product history rather than a release-by-release development journal.

## [1.5.2-beta.5] — Active development

### CodeQL hardening with bounded on-demand work

- Replaced direct weather-secret hashing with a keyed HMAC cache-namespace marker; existing provider cache entries refresh once naturally after the upgrade, without new providers, polling, or key storage.
- Hardened Dashboard Control backup selection around server-discovered regular-file records and reject symlinked backup ZIPs before restore or removal.
- Replaced raw calendar-link backup metadata with trusted-root records: direct `.ics` links may resolve under the Dash-Go user home or the supported system `/Calendars` root, while outside-root, nested, special-file, and unsafe symlink-chain targets fail before a live restore swap. Broken direct calendar links remain supported when their lexical target is within a trusted root.
- Moved runtime font serving to immutable font metadata, verified regular file handles, and `ServeContent`; unchanged downloaded fonts reuse an in-memory size/mtime validation result instead of re-hashing on every font request.
- Removed interpolated calendar-color and theme-preview style attributes in favor of DOM-created nodes and CSS property assignment.
- Replaced dynamic document fallback paths and fragile test-only HTML/URL matching with fixed local assets, literal script extraction, and exact parsed-host checks.
- Added focused source contracts for the addressed CodeQL paths. No dashboard refresh timer, network poll, startup scan, or render-hot-path measurement was added.
- Corrected the config-backup record sort to compare `time.Time` values directly and added a regression test for descending backup ordering with distinct timestamps.

### Reversible day-popup completion

- Made Dash-Go-owned Chore Wheel day-popup checkboxes reversible for current and past assignments: checked means completed and unchecked restores assigned without changing person, date, identity, or fairness planning.
- Replaced the one-way chore completion control with a bounded assigned/completed status mutation while keeping skipped and future assignments read-only.
- Made Maintenance completion state durable in the requested day projection. A mistaken completion can restore its original due date and prior completion value only while it remains the task’s latest safe action.
- Added linked Maintenance undo history, server-side verification, and read-only explanatory rows when a later edit, reschedule, archive, restore, person correction, or newer completion makes restoration unsafe.
- Kept Routine checklist steps reversible after whole-routine completion, and explicitly reject skipped or future routine-session checkbox mutations.
- Updated actionable popup controls to use native checkboxes, server-authoritative rerenders, saving guards, error rollback, touch/keyboard labels, and no browser-memory-only Maintenance completion list.


### Rotating-message safety

- Fixed a Lite-profile footer-edge clipping path where Canvas width prediction could choose too few lines for WebKit’s final wrapping.
- Clamp the message to its selected fitted-line count, so a prediction miss truncates safely inside the fixed footer rather than rendering below the viewport.
- Add one bounded animation-frame verification for each newly generated fit cache entry; an observed wrap mismatch is corrected once, then the repaired result is cached.
- Add a conservative Canvas width allowance, explicit pre-font fallback guard, and a larger multi-line WebKit line-box budget for vertical headroom.
- Extend source and browser message-fit regressions to cover fitted-line clamping and a deliberately induced Lite prediction miss.

### Message catalog and calendar-aware observances

- Reworked the built-in rotating-message catalog around household-safe encouragement, with the requested removals, revisions, and additions.
- Preserve a user’s hidden/default-edit state when a revised built-in message receives new wording; custom messages remain untouched.
- Add event-derived holiday contexts for enabled civil and installer-selected Jewish, Islamic, Christian, Orthodox Christian, and Hindu calendar layers.
- Add neutral acknowledgement, direct celebration greetings, solemn-observance wording, and inclusive overlapping-occasion messages. Direct greetings require both the matching loaded calendar layer and an exact known celebration title; unknown or manually tagged holiday sources receive neutral acknowledgement only.
- Keep holiday-aware rotations mixed with normal household messages: 40% on ordinary observances and 60% on curated major celebrations.

### Household schedules and date corrections

- Added a versioned, local-only Household Schedules model that migrates existing installer-created Payday, Trash Pickup, and Recycling Pickup settings without requiring setup to run again.
- Added multiple named Payday rules with every-N-week, multiple monthly-date, and nth-weekday schedules; monthly dates support 1–31 and use the last day when a requested date does not exist in a month.
- Added per-rule business-day or holiday-shift policies, with weekends and only the user-selected installed holiday layers counted for each rule.
- Added Dashboard Control → Calendars → Household Schedules for recurring edits, pauses, previews, deletion of Payday rules, and restoration of one-time adjustments.
- Added trusted generated-event metadata and a day-popup Manage schedule flow for Dash-Go-owned Paydays, Trash Pickup, and Recycling Pickup only: quick ±1/2/3/7-day moves, a chosen date, skip, and restore normal date.
- Kept imported, subscribed, public holiday, astronomy, and other external calendars read-only; visible titles never determine editability.

### Theme catalog and calendar-aware observance themes

- Expanded the shared theme catalog to 100 curated themes and retired the former More catchall by moving its palettes into Color, Nature & Elements, and Aesthetic groups.
- Added Nature & Elements, Aesthetic, Materials, Practical, and event-backed Holidays & Observances groups while preserving every existing theme ID and saved selection.
- Intentionally excluded Back to School and Game Day.
- Added Memorial Day, Labor Day, Veterans Day, Mother’s Day, Father’s Day, Hanukkah, and Kwanzaa palettes. Hanukkah only appears from a loaded Jewish holiday source with a recognized observance; Kwanzaa requires a recognized event from an enabled holiday-tagged source.
- Kept the theme preview-card presentation, with Seasons fixed to four columns and other groups using responsive four-to-six-column layouts.
- Applied the review’s calendar-legibility corrections to Sunset, Desert, Jade, Firefly, Olive, Cherry, Winter, Independence Day, Thanksgiving, Cinco de Mayo, and Daylight.
- Let the seasonal helper consult the local event cache for exact supported observances before using its existing fixed-date schedule; no new network work or date guessing is introduced.

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

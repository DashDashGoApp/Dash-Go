# Dash-Go Changelog

This changelog records stable Dash-Go milestones. Detailed development increments are consolidated at stable promotion so the file remains useful as a product history rather than a release-by-release development journal.

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

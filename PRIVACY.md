# Dash-Go Privacy

**Applies to:** Dash-Go 1.5.0
**License:** MIT

Dash-Go is designed as a local household dashboard. Its core features work on the device itself and do not require a Dash-Go account, a centralized Dash-Go cloud service, or an advertising service.

This document describes the privacy behavior of an unmodified Dash-Go installation. Optional third-party connections are described in more detail in `INTEGRATIONS.md`.

## Privacy at a glance

- The dashboard control server listens only on `127.0.0.1` by default.
- The kiosk browser communicates with the local dashboard service on the same device.
- Dash-Go does not include a Dash-Go account system, cloud relay, advertising SDK, or analytics collection endpoint.
- Household content remains on the device unless an administrator enables an optional external integration or manually exports, copies, or shares it.
- Dash-Go updates are obtained from the official Dash-Go GitHub repository and GitHub Releases.
- The household administrator is responsible for physical device security, operating-system accounts, network access, backups, and third-party accounts.

## Information stored on the device

Dash-Go can store the following information locally, depending on enabled features:

- Dashboard preferences, display settings, theme choices, performance profile, and configured location coordinates.
- Local calendar files, calendar event details, calendar visibility settings, and cached calendar data.
- Household app data, including people names, To Do and Grocery items, chores, routines, maintenance plans, chalkboard drawings, and Family Message Board content.
- Private Family Message Board records and personal-inbox PIN verifiers.
- Local weather, map, radar, message, and provider-backoff caches.
- Operational data such as update status, health checks, action history, and troubleshooting logs.
- Optional integration settings and secrets, such as calendar credentials, provider API keys, Microsoft authorization data, and notification routes.

Sensitive settings and secrets are kept outside browser-served dashboard content and use owner-only local file permissions where Dash-Go manages those files.

## Information Dash-Go does not send by default

Without an optional integration, Dash-Go does not send household calendar content, task content, family messages, chore assignments, routine data, maintenance records, or dashboard settings to a Dash-Go-operated cloud service.

Dash-Go does not include a mechanism to sell household data, use it for advertising, or build a Dash-Go marketing profile.

## Optional network connections

An administrator may enable optional services for calendars, tasks, weather, maps, radar, notifications, online message content, and font downloads.

When an optional service is enabled, the relevant provider may receive information needed to fulfill that request. Examples include:

- Calendar feed URLs or CalDAV credentials and calendar data.
- Microsoft authorization information and task-list changes.
- Location coordinates for weather, air quality, radar, map, or geocoding requests.
- Event location text when geocoding or a map preview is requested.
- API keys supplied for a selected provider.
- Notification text and configured destination data when an Apprise route sends a notification.
- Standard request information, such as IP address, request time, and user-agent details, that an external provider may log.

Dash-Go does not control the privacy practices, retention, availability, or security of third-party services. Review the provider’s own terms and privacy policy before enabling it.

See `INTEGRATIONS.md` for supported connection types and their local/offline behavior.

## GitHub Releases and updates

Dash-Go uses the official Dash-Go GitHub repository and GitHub Releases for installer, source, and release-download information.

A normal update check or download does not include household calendar content, task content, Family Message Board content, notification routes, provider API keys, or Dashboard Control secrets.

GitHub may receive ordinary HTTPS request information, such as the device IP address, request time, and user-agent details, when the device checks for or downloads a release.

## Local access and device security

Dash-Go is intended for a trusted household network and a locally administered Raspberry Pi or Debian kiosk.

The loopback-only dashboard service reduces network exposure, but it does not protect data from:

- Someone who can sign in to the Dash-Go Linux account.
- A system administrator or root-level user on the device.
- Someone with physical access to an unlocked device or its storage.
- A reverse proxy, port-forwarding rule, or network-binding change added by the administrator.
- Malware or a compromised operating system.

Do not expose the dashboard control server directly to the public internet. Use a strong Linux account password or SSH key, keep the operating system updated, and protect physical access to the device and its storage.

## Backups, exports, and diagnostics

Dash-Go can create local configuration backups for recovery and migration.

A Dashboard Control backup can include configuration data, local calendars, private Family Message Board records, inbox PIN verifiers, configured Apprise route data, and terminal-access settings.

**Dash-Go backups are not encrypted by Dash-Go.** Treat every copied, downloaded, or externally stored backup as sensitive household data. Protect backup files with secure storage and delete obsolete copies deliberately.

Diagnostics, logs, and support exports may contain technical or household context useful for troubleshooting. Review every file before sharing it publicly or sending it to another person. Do not publish private calendar URLs, credentials, tokens, notification routes, backups, or raw configuration files.

## Your choices and controls

The household administrator can:

- Keep Dash-Go entirely local by declining optional integrations.
- Enable, disable, map, or unlink supported integrations.
- Remove local calendar files, app records, messages, and household data through their relevant Dash-Go controls.
- Delete local backups and cached data.
- Change device location, provider choices, and message sources.
- Disable terminal access and use a Dashboard Control PIN for sensitive actions.
- Remove Dash-Go with its offline uninstall workflow.

Disconnecting an integration stops future Dash-Go synchronization with that service. It does not automatically delete local household history, provider-side records, externally stored backups, or data already received by that provider.

## Third-party and administrator responsibilities

The household administrator controls the Dash-Go device, its network, local backups, installed release version, and optional third-party accounts.

If Dash-Go is operated for people outside the administrator’s household, the operator should provide its own contact information, privacy notice, retention policy, and support process appropriate to that deployment.

Third-party software notices are available in `THIRD_PARTY_NOTICES.md`.

## Changes to this document

This document should be updated when a Dash-Go release materially changes what information is stored, sent, retained, or exposed through an optional integration.

For source-level privacy or security concerns, use the project maintainer’s published contact channel. Do not include credentials, private calendar URLs, household exports, or sensitive logs in public reports.

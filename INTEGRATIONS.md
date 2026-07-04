# Dash-Go Integrations

Dash-Go is designed to remain useful as a local household dashboard without an account or cloud connection. Optional integrations add calendar syncing, task syncing, notifications, weather, maps, radar, message content, and optional typography sources. Dash-Go installation and updates are provided through the official Dash-Go GitHub repository and GitHub Releases.

This document describes the integrations available in Dash-Go 1.5.8-beta.1, what they are used for, and the information they may receive. Third-party software licenses and attributions are listed separately in `THIRD_PARTY_NOTICES.md`.

## Local-first operation

Dash-Go does not require a Dash-Go account, central cloud relay, or third-party account for its core household features.

Local calendar files, household apps, people, routines, chores, maintenance plans, messages, Dashboard Control settings, and cached dashboard data remain on the device unless an administrator explicitly configures an outside service.

When an optional service is unavailable, Dash-Go does not invent missing data or silently change local records. Features may show cached content, wait for the next successful refresh, or show an unavailable state.

## Optional integrations at a glance

| Integration | What it provides | Information sent when used | Offline or unlinked behavior |
|---|---|---|---|
| Local iCalendar files | Calendar events from local `.ics` files | Nothing leaves the device | Fully available |
| Remote iCalendar feeds | Read-only calendars from an HTTPS or webcal feed | Feed request; the URL may itself contain a private token | Existing local calendar content remains available until refreshed or removed |
| CalDAV | Calendar synchronization through a compatible CalDAV server; explicit discovery and per-collection display-only or editable selection | CalDAV endpoint, configured credentials, calendar data, and user-requested local-first changes when writeback is enabled | Existing local mirror remains available; writes wait locally for the next successful synchronization |
| Microsoft To Do | Optional task-list synchronization | Microsoft authorization data, mapped list information, and relevant task changes | Local task workflows remain available; remote synchronization waits for recovery |
| Apprise-Go notifications | Optional delivery through configured notification services | Notification text and the configured destination route | No notification is sent while the destination is unavailable |
| Weather and air quality | Forecasts, conditions, air quality, and severe-weather alerts | Configured location coordinates and, where needed, a provider API key | Cached information may remain visible; fresh data cannot be retrieved |
| Radar | On-demand weather radar | Location-derived map tile requests and any configured provider credentials | Radar remains unavailable until a source can be reached |
| Maps and geocoding | Event-map previews, location lookup, and optional interactive maps | Event location text or map coordinates | Local event details remain available without a map image |
| Message feeds | Optional jokes, quotes, facts, riddles, advice, affirmations, and word content | A request to the selected content source; some sources use a configured API key | Local messages and previously cached pulled content remain available |
| Font downloads | Default and optional typography choices | A font-file download request | Dash-Go uses its installed or system fallback fonts |
| GitHub Releases | Dash-Go installer, source, release downloads, and update information | Standard HTTPS request metadata and the requested release asset | The installed dashboard continues running; no update is downloaded |

## Calendar connections

### Local iCalendar files

Dash-Go can display local `.ics` files stored on the device. These files remain entirely local unless an administrator separately synchronizes them with another service.

### Remote iCalendar feeds

A remote iCalendar feed can be added through its URL. Dash-Go requests the feed directly from the configured host.

Treat a private calendar URL as a secret. Some providers embed an access token in the URL itself. Do not place private calendar URLs in screenshots, public issues, source files, or shared configuration exports.

### CalDAV

Dash-Go supports compatible CalDAV workflows through its local synchronization setup. Examples may include self-hosted CalDAV servers and providers that offer CalDAV access.

A CalDAV setup can store an endpoint, account name, app password, token, collection selection, and synchronized calendar data locally on the Dash-Go device. Those credentials are used only to communicate with the configured CalDAV server.

Dashboard writeback is optional and narrow. A connected account is discovered only by a user-led **Discover available calendars** action; discovery creates a review inventory and never changes active syncs. The user then selects each wanted remote collection as display-only or editable. Every selected collection gets an exact local vdir mapping and a separate Dashboard source. A supported Dashboard-created, edited, or deleted event is written locally first and synchronized remotely later for that one selected pair. Eligible recurring events can make a local exception for one occurrence or update a simple series without rewriting its repeat rule; advanced rules, attendee/organizer events, and ambiguous existing exception sets remain provider-managed. URL subscriptions, broad legacy multi-collection mirrors, generated feeds, and unmanaged local ICS files are never writeback targets. Dash-Go never deletes a remote calendar; stopping future synchronization preserves the local mirror by default.

Conflicts are safe-stop behavior. When the same event changes locally and remotely before a sync, Dash-Go keeps both sides intact, marks only that selected source as needing attention, and does not automatically choose a winner. The dashboard continues to show its local version. Scheduled-sync outcomes are summarized into the Calendar Manager without exposing raw vdirsyncer logs. A selected exact pair that needs initial discovery can use **Repair connection**, which performs one targeted discovery and sync without changing selected calendars.

A current conflict can be resolved only from Dashboard Control after the administrator has configured and unlocked a Dashboard Control PIN. The confirmation clearly states that the choice applies to every unresolved conflict in that one calendar pair: **Use phone / remote version** retains the remote side, while **Use this dashboard's version** retains the local Dash-Go side. Dash-Go creates an owner-only, bounded local snapshot before a remote-winner run, but that snapshot is not a remote backup and cannot restore a remote version that has been deliberately replaced. The selected winner is used in one temporary targeted vdirsyncer configuration, then removed; normal scheduled sync configuration never retains an automatic conflict winner. Removing the last local event of a collection is propagated only for the one deletion the dashboard itself performed; an unexpectedly emptied collection is never pushed to a provider.

Timed private-calendar event forms also offer local touch time nudges and length presets. They only rewrite the ordinary visible local form fields before the existing local-first save path runs; they do not add a new remote service or background synchronization route.

### Google Calendar

Google Calendar synchronizes through the same vdirsyncer pipeline using Google's CalDAV endpoint with OAuth. One Google connection can own multiple selected calendar mappings while reusing its private OAuth token. Exact selected Google collections can be display-only or, when Dashboard edits are enabled, support local-first creation, one-time management, one-occurrence recurring changes, and constrained simple-series management. A write queues only the affected Google collection; a failure, revoked token, or conflict leaves other selected calendars available. Dashboard Control identifies a Google authorization issue and directs the administrator to retry the owner-only private-calendar authorization flow; it never displays OAuth material. Dash-Go never deletes a Google calendar.

### Managed vdirsyncer installation

Dash-Go manages private-calendar synchronization through one isolated **pipx** environment pinned to `vdirsyncer[google]` **0.20.0**. On Raspberry Pi OS, Debian, and Ubuntu it may use APT only to install `pipx`; the vdirsyncer application and its Google OAuth dependency are then installed together in `~/.dashboard-vdirsyncer/pipx/` with its known command wrapper under `~/.dashboard-vdirsyncer/bin/`. Dash-Go never uses `pip --user`, never installs vdirsyncer into the system Python environment, and never runs `pipx upgrade` automatically.

This is an optional, short-lived external sync tool: it is not a Dash-Go server dependency or daemon. Python runs only for vdirsyncer dependency checks, private-calendar discovery, or synchronization. Dash-Go’s one-time Google authorization helper is written in Go; it writes the standard OAuth token mapping that the isolated vdirsyncer environment subsequently refreshes during ordinary sync. Scheduled, manual, setup, and queued writeback syncs all inherit Dash-Go’s low CPU/I/O priority; completed sync processes exit and leave no retained Python service. Remote collection discovery is explicit setup/refresh work rather than a recurring 15-minute task, so reopen private-calendar setup after adding a new remote calendar or changing a collection list. Setup checks the actual APT, pipx, Google, iCloud, or chosen CalDAV operation when it needs network access; it does not use an unrelated weather service as a generic internet test. If a system has no APT, setup requires an administrator to install `pipx` with that system’s native package manager before private-calendar configuration can continue. On Debian Bullseye, Dash-Go uses `bullseye-backports` for `pipx` and `python3-venv` before creating its isolated environment.

Google requires a one-time preparation in your own Google Cloud account, because Google does not allow password-based CalDAV access:

1. Create (or reuse) a Google Cloud project and enable the **CalDAV API** for it.
2. Configure the OAuth consent screen. For a personal or household account, set the publishing status to **In production**; a client left in *Testing* status receives refresh tokens that Google expires after seven days, which would silently stop background sync.
3. Create an OAuth client ID of type **Desktop app** and note its client ID and client secret.

Run `setup-vdirsyncer.sh`, choose the Google Calendar provider, and supply the client ID and secret. The client secret and the OAuth token are stored outside the dashboard webroot in `~/.dashboard-vdirsyncer/` with owner-only permissions, exactly like CalDAV app passwords. Then use Dashboard Control’s discovery inventory to review the account collections and select the ones Dash-Go should synchronize. You do not need to type or remember Google calendar IDs in normal use; Dash-Go keeps those opaque IDs as provider metadata and uses generated safe local collection keys.

Authorization happens once per enrolled Google account at setup time through the Go-native dashboard control server helper. It uses PKCE, verifies a one-time state value, and atomically stores the owner-only token at `~/.dashboard-vdirsyncer/google-tokens/<connection>.json`, which vdirsyncer then refreshes during ordinary sync. Dash-Go never asks vdirsyncer to open its own interactive browser flow.

**Desktop app (recommended and universal):** choose **desktop** when creating the Google OAuth client. Dash-Go prints a sign-in link and, when `qrencode` is installed, an optional terminal QR code. Open the link on any phone or computer. After approval, the browser follows the Desktop OAuth loopback redirect to `http://127.0.0.1:8433/`; on a remote phone or computer that page may show a connection error because its own loopback address has no Dash-Go listener. Copy the **complete** address from the browser address bar and paste it into the terminal. No public callback, SSH tunnel, or additional Dash-Go service is required.

**Web app (automatic kiosk QR completion):** choose **web** only when an administrator already has an exact HTTPS reverse-proxy callback that forwards to the Dash-Go device. Before creating the Google Web client, setup prints the exact URI to register, ending in `/oauth/google/callback`. While the SSH authorization is armed, Dash-Go shows a QR code on the kiosk display; scan it, sign in, and the callback completes automatically. The callback is available only during the one-shot armed window, and the display disappears after success, timeout, or cancellation. Dash-Go itself remains loopback-only; do not expose its general control API or use a plain HTTP LAN callback. When no exact HTTPS callback is available, use **desktop** instead.

Run `~/dashboard/bin/setup-vdirsyncer.sh --authorize` to retry saved Google connections. That focused pass does not add calendar names, run a regular sync, or change cron; it only authorizes eligible saved connections, performs their ordinary setup-time discovery, and refreshes Dash-Go’s generated private-calendar files. Add `-no-display` only when invoking the `--google-oauth` helper directly and you deliberately do not want the kiosk QR overlay.

An enrolled Google calendar whose authorization has not completed is simply skipped by scheduled synchronization (with a safe Calendar Manager status) rather than blocking it; its previous local data remains displayed, and it joins the next run once authorized. Token refresh afterwards is automatic and unattended. If Google access is revoked from the account's security settings, synchronization for that calendar stops with a provider-specific authorization-needed state while the rest of the dashboard continues normally; re-run `~/dashboard/bin/setup-vdirsyncer.sh --authorize` to restore it. For iCloud and other CalDAV sources, Calendar Manager instead advises the administrator to update the account or app password through private-calendar SSH setup.

Before a private-calendar configuration is treated as stable, validate real provider behavior with an owned calendar, an editable shared calendar, a read-only shared calendar, duplicate display names, a rename after selection, an authorization failure, a same-event conflict, each explicit winner choice, a connection repair, and normal cron synchronization after recovery. Repeat the normal create/edit/delete, recurring occurrence/series edit, and conflict checks for iCloud or another compatible CalDAV provider. These physical-account checks complement, rather than replace, the local builder and source tests.

## Microsoft To Do

Microsoft To Do is optional and is connected through Microsoft Graph using an explicit device authorization flow.

Dash-Go may request authorization, read available task-list information during setup, and synchronize the list that the administrator maps to Dash-Go. Relevant task titles, completion state, and task changes are sent to or received from Microsoft only when that connection is configured.

Dash-Go keeps the household task experience usable when Microsoft To Do is disconnected. Remote changes wait for a later successful sync rather than replacing local household workflows with an error screen.

Disconnecting Microsoft To Do stops future synchronization. It does not silently delete local Dash-Go task history, grocery-memory suggestions, or household app data.

## Notifications through Apprise-Go

Dash-Go can use Apprise-Go to send configured notifications through supported third-party destinations, such as email, chat, push, or other service routes supported by the configured Apprise destination.

Notification routes are configured locally through SSH and are not exposed in Dashboard Control. A route can contain sensitive destination addresses, tokens, webhook URLs, or credentials.

When a notification is sent, the configured destination receives the notification content and the information necessary to deliver it. Dash-Go does not operate a notification relay or store these routes in browser assets.

## Weather, air quality, and severe-weather alerts

Weather features use the dashboard’s configured location coordinates. Open-Meteo is the default forecast source. Additional supported forecast sources may include:

- Open-Meteo
- WeatherAPI
- OpenWeather
- Google Weather
- Tomorrow.io
- Visual Crossing
- Weatherbit
- Pirate Weather
- AccuWeather
- Xweather
- National Weather Service, where supported

Some providers require an API key. A configured key is stored locally and is sent only to that provider when making its request.

Dash-Go can also request air-quality data and National Weather Service severe-weather alerts. National Weather Service alert coverage is limited to areas supported by that service.

Provider availability, terms, quotas, pricing, and rate limits are controlled by the provider. Dash-Go applies bounded refresh behavior and provider backoff, but it cannot guarantee a provider’s availability or retention policy.

## Radar

Radar is an on-demand feature rather than a continuously running service. Opening radar may request map tiles or frames associated with the dashboard’s configured location and current viewport.

RainViewer is the standard public radar source. Dash-Go can also use supported provider routes, including National Weather Service, Tomorrow.io, Weatherbit, Xweather, or an administrator-configured public HTTPS tile or WMS source.

Lite profiles keep radar work bounded for Raspberry Pi Zero 2 W reliability. Radar loading or recovery is not treated as a household-data failure.

## Maps, geocoding, and interactive location links

Dash-Go can create map previews for calendar-event locations and can geocode an event’s location text.

Depending on availability and selected map style, Dash-Go may use:

- OpenStreetMap standard, HOT, or Germany tile services
- StaticMap DE
- Esri imagery and boundary-label services
- OpenStreetMap Nominatim
- United States Census geocoding
- Open-Meteo geocoding

A geocoding request can include an event’s location text. A map-image or tile request includes coordinates and map viewport information.

When enabled, an interactive location action can open Google Maps in the local kiosk browser. This is a user-initiated action and is separate from Dash-Go’s cached map-preview system.

## Optional message feeds

Dashboard Control can enable optional online message categories such as jokes, quotes, facts, riddles, advice, affirmations, and word content.

Dash-Go only contacts sources selected by the administrator. Current source options may include public services such as API Ninjas, icanhazdadjoke, JokeAPI, Official Joke API, Quotable, FavQs, ZenQuotes, type.fit, DummyJSON, Useless Facts, Cat Facts, Meow Facts, Numbers API, Riddles API, Affirmations.dev, Advice Slip, and Random Word API.

Some providers may require an API key. Pulled text can be cached locally and can be edited or removed in Dashboard Control.

External content is supplied by its respective source. Dash-Go does not guarantee its accuracy, suitability, availability, or retention.

## Font downloads

Dash-Go installs or downloads application fonts from pinned sources during setup and only downloads optional font choices after the administrator selects them.

The standard Dash-Go interface uses Libre Franklin and DM Mono. Optional typography choices can include Nunito and Atkinson Hyperlegible.

When a font source cannot be reached, Dash-Go uses the installed or system fallback font stack. A font-download request exposes ordinary network metadata, such as the device’s IP address, to the font host.

## GitHub Releases and updates

Dash-Go uses the official [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases for installation, source access, release downloads, and update information.

When Dash-Go checks for or downloads an update, GitHub may receive ordinary HTTPS request information, such as the device IP address, request time, and user-agent details. A normal update check or release download does not include household calendar content, task content, Family Message Board content, notification routes, provider API keys, or Dashboard Control secrets.

Dash-Go release downloads are public project assets. Do not place private household data, credentials, calendar URLs, backups, or diagnostic exports in GitHub Releases, public issues, pull requests, discussions, or repository commits.

An update is staged and validated before managed application files are replaced. A failed update should leave the existing dashboard running.

## Administrator responsibilities

Before enabling an optional service, the administrator is responsible for reviewing that provider’s:

- Terms of service
- Privacy policy
- Account, API-key, and billing requirements
- Geographic coverage and availability
- Rate limits and retention practices

The names of third-party services are used only to identify supported connections. Dash-Go is not affiliated with, endorsed by, sponsored by, or responsible for those services unless explicitly stated by the service owner.

## Keeping this inventory current

Update this document whenever Dash-Go adds, removes, materially changes, or changes the default behavior of an external integration.

Do not place credentials, private feed URLs, notification routes, account identifiers, access tokens, or household data in this document.

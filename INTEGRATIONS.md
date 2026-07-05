# Dash-Go Integrations

## Supported Debian-family bases

Dash-Go fresh installs support Debian or Raspberry Pi OS **Bookworm** and recommend **Trixie**. Bullseye and older are blocked before package, service, kiosk, or display changes; use Doctor, backup, or uninstall while preparing an upgrade. Newer releases run in conservative mode, so Dash-Go avoids unreviewed display changes.


Dash-Go is designed to remain useful as a local household dashboard without an account or cloud connection. Optional integrations add calendar syncing, task syncing, notifications, weather, maps, radar, message content, and optional typography sources. Dash-Go installation and updates are provided through the official Dash-Go GitHub repository and GitHub Releases.

This document describes the integrations available in Dash-Go 1.5.8-beta.7, what they are used for, and the information they may receive. Third-party software licenses and attributions are listed separately in `THIRD_PARTY_NOTICES.md`.

## Local-first operation

Dash-Go does not require a Dash-Go account, central cloud relay, or third-party account for its core household features.

Local calendar files, household apps, people, routines, chores, maintenance plans, messages, Dashboard Control settings, and cached dashboard data remain on the device unless an administrator explicitly configures an outside service.

When an optional service is unavailable, Dash-Go does not invent missing data or silently change local records. Features may show cached content, wait for the next successful refresh, or show an unavailable state.

## Optional integrations at a glance

| Integration | What it provides | Information sent when used | Offline or unlinked behavior |
|---|---|---|---|
| Local iCalendar files | Calendar events from local `.ics` files | Nothing leaves the device | Fully available |
| Remote iCalendar feeds | **Read-only** calendars from an HTTPS or webcal feed; Dash-Go never writes to the provider | Feed request; the URL may itself contain a private token | Existing local calendar content remains available until refreshed or removed |
| CalDAV | Calendar synchronization through a compatible CalDAV server; explicit discovery and per-collection **view-only** or **two-way** selection | CalDAV endpoint, configured credentials, calendar data, and user-requested local-first changes only when two-way sync is enabled | Existing local mirror remains available; writes wait locally for the next successful synchronization |
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

A remote iCalendar feed can be added through installer option **9) Read-only calendar link**. Dash-Go requests the feed directly from the configured host and treats it as a one-way display source: it can show events but can never add, edit, delete, or skip provider events through that link.

Treat a private calendar URL as a secret. Some providers embed an access token in the URL itself. Do not place private calendar URLs in screenshots, public issues, source files, or shared configuration exports.

### Personal calendar access and safe mode changes

Installer option **10) Personal calendar sync** is the signed-in route. It supports a per-calendar choice between **view-only** and **two-way sync**. View-only keeps provider events visible but makes Dashboard Control refuse edits, removes the calendar from the writeback registry, and generates a read-only provider policy. Two-way sync is an explicit opt-in; Dash-Go verifies a targeted provider sync before edit controls are made available.

Switching a signed-in iCloud or CalDAV calendar from two-way to view-only immediately stops queued writes, takes an owner-only local snapshot, and preserves the account connection for future reads. Switching back verifies the exact selected provider calendar before Dash-Go restores writeback. Google has the same immediate secure-connection safety lock. To replace a Google secure source with a true iCal link, first switch it view-only in Calendar Manager, add and verify the link through option 9, then stop the old secure source; Dash-Go does not silently merge or duplicate data sources.

### CalDAV

Dash-Go supports compatible CalDAV workflows through its local synchronization setup. Examples may include self-hosted CalDAV servers and providers that offer CalDAV access.

A CalDAV setup can store an endpoint, account name, app password, token, collection selection, and synchronized calendar data locally on the Dash-Go device. Those credentials are used only to communicate with the configured CalDAV server.

Dashboard writeback is optional and narrow. A connected account is discovered only by a user-led **Discover available calendars** action; discovery creates a review inventory and never changes active syncs. The user then selects each wanted remote collection as **view-only** or **two-way**. Every selected collection gets an exact local vdir mapping and a separate Dashboard source. A supported Dashboard-created, edited, or deleted event is written locally first and synchronized remotely later for that one selected pair. Eligible recurring events can make a local exception for one occurrence or update a simple series without rewriting its repeat rule; advanced rules, attendee/organizer events, and ambiguous existing exception sets remain provider-managed. URL subscriptions, broad legacy multi-collection mirrors, generated feeds, and unmanaged local ICS files are never writeback targets. Dash-Go never deletes a remote calendar; stopping future synchronization preserves the local mirror by default.

Conflicts are safe-stop behavior. When the same event changes locally and remotely before a sync, Dash-Go keeps both sides intact, marks only that selected source as needing attention, and does not automatically choose a winner. The dashboard continues to show its local version. Scheduled-sync outcomes are summarized into the Calendar Manager without exposing raw vdirsyncer logs. A selected exact pair that needs initial discovery can use **Repair connection**, which performs one targeted discovery and sync without changing selected calendars.

A current conflict can be resolved only from Dashboard Control after the administrator has configured and unlocked a Dashboard Control PIN. The confirmation clearly states that the choice applies to every unresolved conflict in that one calendar pair: **Use phone / remote version** retains the remote side, while **Use this dashboard's version** retains the local Dash-Go side. Dash-Go creates an owner-only, bounded local snapshot before a remote-winner run, but that snapshot is not a remote backup and cannot restore a remote version that has been deliberately replaced. The selected winner is used in one temporary targeted vdirsyncer configuration, then removed; normal scheduled sync configuration never retains an automatic conflict winner. Removing the last local event of a collection is propagated only for the one deletion the dashboard itself performed; an unexpectedly emptied collection is never pushed to a provider.

Timed private-calendar event forms also offer local touch time nudges and length presets. They only rewrite the ordinary visible local form fields before the existing local-first save path runs; they do not add a new remote service or background synchronization route.

### Google Calendar

Google offers two deliberate Dash-Go routes. Use installer option **9) Read-only calendar link** for a view-only Google iCal/ICS address. Use option **10) Personal calendar sync** for signed-in Google OAuth and two-way synchronization. One Google account can own several exact selected mappings while reusing one owner-only OAuth token. An existing signed-in Google calendar can be locked view-only from Calendar Manager immediately; converting it to a separate iCal link remains a staged, verified source change so Dash-Go never guesses which similarly named calendar should replace it.

Private-calendar setup is a guided account-first flow. Choose **Google Calendar**, let Dash-Go check its private calendar tool, then create one OAuth client in your own Google Cloud project. Dash-Go tells you exactly what to select: enable Google Calendar API, configure the consent screen, and create an OAuth Client ID with application type **Desktop app**. “Desktop app” is Google’s label for this secure local sign-in method; it does not mean the dashboard itself must be a desktop computer. The normal wizard never asks you to choose “desktop” versus “web.”

The recommended completion path uses a temporary SSH bridge. Leave the setup terminal open, open a second terminal on the computer you used for SSH, and paste the one `ssh -N -L ...` command Dash-Go prints. Open the sign-in link in that same computer’s browser. Dash-Go temporarily listens only on its own `127.0.0.1:8433` address, receives the result through that bridge, and continues automatically. The listener closes after completion, cancellation, or timeout.

When setup is running through headless SSH, Dash-Go defaults to **Phone or tablet** so “this computer” never means the remote Pi by accident. It can show the Google sign-in QR code on the kiosk display and prints the same link in the terminal. After approval, the phone may show a connection-error page at `127.0.0.1`; that is expected in this fallback. Copy the complete address from the browser bar and paste it into the setup terminal. Dash-Go does not claim that this phone route completes automatically. If `qrencode` is unavailable, Dash-Go says so and still prints the link.

Setup trims pasted credentials, accepts `q` as a safe cancellation at each private-calendar question, and re-asks only a bad field or calendar selection. A wrong selection does not throw away a successful Google sign-in or an already discovered account.

Dash-Go stores the client secret and OAuth token only under `~/.dashboard-vdirsyncer/` with owner-only permissions. The Go-native helper uses PKCE, verifies one random state value, and writes the token mapping vdirsyncer refreshes during normal sync. Dash-Go never asks vdirsyncer to open its own interactive browser flow. Use `~/dashboard/bin/setup-vdirsyncer.sh --authorize` to reconnect an existing Google account without entering names, colors, or new calendar selections.

Keep a household Google project out of the temporary testing state when Google permits it. A project left in testing can lose unattended refresh access after about seven days, which makes regular calendar sync unreliable. Google Calendar setup then discovers the actual calendars, lets the administrator choose them by their human-readable names, preserves an optional display color, creates the selected secure source for two-way sync, performs a first sync, and only then activates scheduled synchronization. Normal setup never asks for a Google Calendar ID.

### Apple iCloud Calendar

Apple iCloud Calendar is a first-class private-calendar option, not a generic CalDAV form. Choose **Apple iCloud Calendar** and Dash-Go asks only for the Apple Account email and a newly generated Apple app-specific password. Do not enter the normal Apple Account password. Dash-Go uses the fixed iCloud CalDAV endpoint internally and never asks ordinary users for a server URL or collection UUID.

Create the Apple app-specific password at `account.apple.com` under **Sign-In and Security** → **App-Specific Passwords**. Name it something recognizable such as `Dash-Go Calendar`, then paste it into the masked prompt. The password is stored only in Dash-Go’s owner-only private calendar folder and is never printed, placed in a URL, or passed as a command-line argument.

Dash-Go tests the account in a private draft, discovers the available iCloud calendars, and lets the administrator choose familiar names such as Family or Birthdays. Each selected iCloud calendar can stay **view-only** or use **two-way sync** through the same Apple account connection. Two-way/writeback permission is an explicit per-calendar choice and should normally be limited to calendars the household owns. Dash-Go performs a first sync before it activates the connection or creates the scheduled sync wrapper. Cancelling or a failed first sync leaves existing dashboard calendars, cron state, and private mappings unchanged.

If an app-specific password is revoked or the primary Apple Account password changes, Calendar Manager reports that the iCloud connection needs a new app-specific password and directs the administrator to replace it through private-calendar setup. Setup trims pasted values, gives a non-blocking example of the usual `abcd-efgh-ijkl-mnop` app-password shape, and translates iCloud authorization, account/2FA, and network failures into a concrete next action. Dash-Go does not create remote iCloud calendars; create those in Apple Calendar or iCloud’s own web interface.

### Managed vdirsyncer installation

Dash-Go manages private-calendar synchronization using a pinned `vdirsyncer[google]` **0.20.0** tool outside the dashboard webroot. Setup uses the safest available route in this order:

1. Reuse a healthy Dash-Go-managed tool without prompting.
2. Use an existing working `pipx` installation to create Dash-Go’s isolated environment.
3. When `pipx` is missing or broken, install or repair `pipx` and Python virtual-environment support through the system package manager on a supported Bookworm/Trixie device.
4. When pipx cannot be used, create an isolated Dash-Go-owned virtual environment under `~/.dashboard-vdirsyncer/pip-fallback-venv/`.
5. Only after explicit confirmation, use a last-resort per-user `python3 -m pip install --user` compatibility fallback.

Setup distinguishes a working `pipx` installation from an absent Dash-Go vdirsyncer environment. It does not offer to repair pipx merely because the Dash-Go calendar tool has not yet been installed. The final user-level pip route never uses `sudo pip`, never uses `--break-system-packages`, and never changes the operating system Python packages.

The tool is optional and short-lived: it is not a Dash-Go server dependency or daemon. Python runs only for vdirsyncer dependency checks, private-account discovery, or synchronization. Scheduled/manual setup and queued writeback syncs inherit Dash-Go’s low CPU/I/O priority; completed sync processes exit and leave no retained Python service. Discovery is explicit account setup or repair work rather than a recurring 15-minute task.

### Compatible CalDAV accounts

For Nextcloud, Fastmail, Radicale, Baïkal, and similar services, choose **Another CalDAV account**. That advanced-compatible path asks for an account label, CalDAV URL, username, and password or provider app password. It follows the same private draft, discovery-before-selection, read-only-by-default, first-sync-before-activation, and cancellation safety rules as Google and iCloud.

Before treating a private-calendar configuration as stable, validate real provider behavior with an owned calendar, an editable shared calendar, a read-only shared calendar, duplicate display names, a rename after selection, authorization failure, same-event conflict, each explicit winner choice, a connection repair, and normal cron synchronization after recovery. These physical-account checks complement local builder and source tests.

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

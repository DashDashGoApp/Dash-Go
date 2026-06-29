# Dash-Go Integrations

Dash-Go is designed to remain useful as a local household dashboard without an account or cloud connection. Optional integrations add calendar syncing, task syncing, notifications, weather, maps, radar, message content, and optional typography sources. Dash-Go installation and updates are provided through the official Dash-Go GitHub repository and GitHub Releases.

This document describes the integrations available in Dash-Go 1.5.0, what they are used for, and the information they may receive. Third-party software licenses and attributions are listed separately in `THIRD_PARTY_NOTICES.md`.

## Local-first operation

Dash-Go does not require a Dash-Go account, central cloud relay, or third-party account for its core household features.

Local calendar files, household apps, people, routines, chores, maintenance plans, messages, Dashboard Control settings, and cached dashboard data remain on the device unless an administrator explicitly configures an outside service.

When an optional service is unavailable, Dash-Go does not invent missing data or silently change local records. Features may show cached content, wait for the next successful refresh, or show an unavailable state.

## Optional integrations at a glance

| Integration | What it provides | Information sent when used | Offline or unlinked behavior |
|---|---|---|---|
| Local iCalendar files | Calendar events from local `.ics` files | Nothing leaves the device | Fully available |
| Remote iCalendar feeds | Read-only calendars from an HTTPS or webcal feed | Feed request; the URL may itself contain a private token | Existing local calendar content remains available until refreshed or removed |
| CalDAV | Calendar synchronization through a compatible CalDAV server | CalDAV endpoint, configured credentials, and calendar data | No new remote changes are synchronized |
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

Removing or disabling a CalDAV connection stops future synchronization. It does not automatically erase local household data or unrelated calendar files.

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

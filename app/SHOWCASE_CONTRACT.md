# Dash-Go Showcase Contract v1

Dash-Go Showcase Contract v1 is an **inactive-by-default** runtime boundary for Dash-Go Showcase Studio. A normal Dash-Go install does not enter Showcase mode, does not read a scenario manifest, and continues to use its ordinary app and user-data paths.

Studio enables the profile only for a disposable local scenario by supplying all three variables:

```text
DASHGO_RUNTIME_PROFILE=showcase
DASHGO_SHOWCASE_MANIFEST=/absolute/path/to/showcase-manifest.json
DASHGO_DATA_ROOT=/absolute/path/to/disposable-scenario-root
```

`DASHGO_DATA_ROOT` must be absolute, separate from the immutable Dash-Go application directory, and own the scenario manifest. Dash-Go uses it for mutable scenario configuration, calendars, cache, logs, and the private scenario home under `home/`. It never uses the operator's real Dash-Go home directory while this profile is active.

The shipped machine-readable declaration is `release/showcase-contract.json`. Its contract identifier is `dashgo-showcase/v1`.

## Scenario manifest

The manifest is strict JSON with no unknown fields. Studio owns it and stores it below the data root.

```json
{
  "schema": 1,
  "contract": "dashgo-showcase/v1",
  "profile": "showcase",
  "scenario": "everyday-household",
  "cache": {
    "daysPast": 30,
    "daysFuture": 180
  },
  "calendars": [
    {
      "source": "calendars/family.green.ics",
      "name": "Family",
      "color": "#3aa981",
      "collection": "home/.dashboard-vdirsyncer/collections/family",
      "writable": true,
      "enabled": true,
      "expectedEvents": 6
    }
  ]
}
```

A calendar `source` must use the `calendars/<name>.ics` form. Each collection must remain under `home/.dashboard-vdirsyncer/collections` in the disposable data root. The manifest is the sole source of the Showcase calendar manifest and calendar-writeback registry.

At profile startup Dash-Go validates the contract and manifest, writes the scenario calendar manifest, registers the declared private writable calendars, and rebuilds the event cache within the supplied window.

## Readiness proof

`GET /api/showcase/status` is the authoritative readiness report. It includes the selected contract/profile, immutable app and disposable data roots, every declared calendar, fixture presence, cached event and writeback-candidate counts, cache readiness, and a bounded problem list.

Studio must refuse to open the browser when this report is not ready. A successful HTTP response alone is not sufficient.

`dashboard-control-server --showcase-contract` prints the shipped contract declaration for source and packaging validation.

## Scope of v1 vertical slice

This first Contract v1 slice establishes isolated roots, scenario calendars, cache/readiness status, and the browser-visible scenario data allowlist. It does not itself provide Studio's offline weather/map/geocode fixtures, presentation extension assets, or the complete Showcase action policy; those migrate in the next contract increment. Until then Studio retains its legacy overlay path for released Dash-Go versions that do not expose the contract.

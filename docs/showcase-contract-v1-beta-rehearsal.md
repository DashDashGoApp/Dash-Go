# Dash-Go 1.5.9-beta.1 — Showcase Contract v1 Rehearsal

This beta is the first Dash-Go source handoff that ships `dashgo-showcase/v1`.

The local Builder remains responsible for compiled binaries, generated browser assets, package validation, checksums, SBOM, and release assets. After the exact beta is published, Dash-Go Showcase Studio must stage that exact release asset and prove the **native** Contract v1 runtime path. The expected native sequence is:

1. Studio detects `app/release/showcase-contract.json`.
2. Studio seeds `showcase-manifest.json` below its disposable scenario data root.
3. Studio starts Dash-Go with `DASHGO_RUNTIME_PROFILE=showcase`, `DASHGO_SHOWCASE_MANIFEST`, and `DASHGO_DATA_ROOT`.
4. Dash-Go rebuilds the bounded cache, registers only the manifest-declared writable calendars, and reports readiness at `/api/showcase/status`.
5. Studio refuses browser launch unless the status is ready, reports all four scenario calendars, and includes a writeback candidate for each writable calendar.

A successful legacy overlay path is not proof for this beta. Legacy remains a fallback only for packaged releases that do not declare Contract v1.

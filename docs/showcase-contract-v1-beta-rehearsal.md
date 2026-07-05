# Dash-Go 1.5.9-beta.3 — Showcase Contract v1 Rehearsal

This beta is the first Dash-Go source handoff that ships `dashgo-showcase/v1`.

The local Builder remains responsible for compiled binaries, generated browser assets, package validation, checksums, SBOM, and release assets. After the exact beta is published, Dash-Go Showcase Studio must stage that exact release asset and prove the **native** Contract v1 runtime path. The expected native sequence is:

1. Studio detects `app/release/showcase-contract.json`.
2. Studio seeds `showcase-manifest.json` below its disposable scenario data root.
3. Studio starts Dash-Go with `DASHGO_RUNTIME_PROFILE=showcase`, `DASHGO_SHOWCASE_MANIFEST`, and `DASHGO_DATA_ROOT`.
4. Dash-Go rebuilds the bounded cache, registers only the manifest-declared writable calendars, and reports readiness at `/api/showcase/status`.
5. Studio refuses browser launch unless the status is ready, reports all four scenario calendars, and includes a writeback candidate for each writable calendar.

A successful legacy overlay path is not proof for this beta. Legacy remains a fallback only for packaged releases that do not declare Contract v1.

## Beta.3 Windows static-data-routing correction

The first Contract v1 beta selected the native Studio path and then stopped at the Windows engine cross-compile. Beta.2 moved the remaining Unix-only platform primitives behind Dash-Go-owned build-tagged helpers and added the Builder-owned Windows server cross-compile gate.

The beta.2 native Stage proof then passed, including Contract status, cache rebuilding, and four writable scenario calendars. Its installed Windows Studio self-test exposed a later boundary: an ordinary HTTP request for `/config/config.local.js` reached Dash-Go's static handler, which used host-filesystem path cleaning before native Showcase data allowlisting. A Windows build changed the URL slashes into backslashes, making the otherwise allowlisted scenario fixture look like a different path and returning 404 from the immutable app root.

Beta.3 canonicalizes static request paths with URL slash semantics before any filesystem join or native Showcase allowlist check. It includes a source test with Windows-shaped separators and an installed-route test for `config.local.js`, so the Builder's ordinary Go suite rejects a recurrence before package publication. The Builder 1.0.40 Windows cross-compile gate remains required and unchanged.

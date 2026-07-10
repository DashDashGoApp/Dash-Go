# Releasing Dash-Go

Dash-Go is a public GitHub project. GitHub Releases are the canonical installer and update distribution channel. The release workflow is deliberately fail-closed: source is audited, the local builder creates every generated asset, GitHub publication starts as a draft, and the public route is probed only after publication.

The local Windows/WSL builder owns generated browser bundles, Linux binaries, release archives, binary-derived SPDX SBOMs, checksums, and release catalogs. Do not hand-edit an archive, SBOM, `SHA256SUMS`, or catalog after a successful build.

## Release identity and assets

Every release contains exactly these assets:

```text
Dash-Go_X.Y.Z[-beta.N]_release.tar.gz
Dash-Go_X.Y.Z[-beta.N]_source.tar.gz
Dash-Go_X.Y.Z[-beta.N]_sbom.spdx.json
SHA256SUMS
```

- Stable tag: `vX.Y.Z`; GitHub release is **not** a prerelease.
- Beta tag: `vX.Y.Z-beta.N`; GitHub release **is** a prerelease.
- Published releases and tags are immutable. Correct a published release with a new version; never replace its assets or force-push its tag.
- Before a stable promotion, collapse the active `X.Y.Z-beta.*` entries in `CHANGELOG.md` into one `X.Y.Z` stable section, preserving the user-visible changes but removing the interim beta headings.

## 1. Prepare the source repository

Start with the final source handoff, not a builder directory or a prior release-asset directory.

1. Extract the handoff into the dedicated Dash-Go Git working tree.
2. Confirm it contains `app/tests/run-all.sh`, `.github/workflows/verify.yml`, and this `RELEASING.md`; the source runner, CI workflow, and maintainer instructions are one handoff contract and may not be filtered independently. Also confirm it contains no `AI.md`, generated binaries, browser bundles, release assets, logs, caches, backups, calendars, credentials, or local configuration.
3. Run the release gate from the source root: `app/tests/run-all.sh`. It requires clean `gofmt`, a no-diff `go mod tidy`, Go module verification, `go vet`, `go test ./...`, exactly ShellCheck 0.9.0 with error-severity findings release-blocking, and every shell/Node smoke. ShellCheck warnings remain review findings and must be resolved or narrowly documented when they identify real dead or indirect state. The GitHub Actions **Verify Dash-Go source** workflow must be green for the same commit.
4. Exercise the beta.6 selector migration in that gate (`beta6-selector-upgrade-smoke.sh`). For an actual device still on **1.5.8-beta.6**, extract the verified **1.5.8** release bundle and run `./install.sh --bootstrap-selector-integrity` before its first ordinary stable update. The bridge atomically refreshes `~/install.sh`, verifies the copied bytes, then starts the corrected stable updater; it is not needed for newer installs.
5. Inspect `git status`, `git diff --cached`, `.gitignore`, and `.gitattributes` before committing.
6. Commit only the reviewed source tree to `DashDashGoApp/Dash-Go`.

## 2. Build release assets locally

Run the local builder from its established fixed folder:

```powershell
Set-Location 'C:\Users\chris\Projects\Calendar\Dash-Go_Local_Builder_1.0.3'
.\Build-DashGoRelease.ps1 -Performance Max
```

Select the exact source handoff and allow the Builder to complete every gate. On success it creates the release-asset directory used by the Publisher. Do not change files in that directory.

A same-version beta rebuild is allowed only before publication, only with the Builder’s explicit `-Force` confirmation, and never as a way to bypass a failed validation. Stable releases are never rebuilt or overwritten at the same version.

## Showcase Contract v1 beta rehearsal

When a beta source handoff includes `app/release/showcase-contract.json`, it must complete the normal local Builder and GitHub beta-release process first. Then stage the **exact published GitHub Release asset** in Dash-Go Showcase Studio. Studio must detect `dashgo-showcase/v1`, launch Dash-Go with `DASHGO_RUNTIME_PROFILE=showcase`, seed its four disposable writable calendars, and require `/api/showcase/status` to report `ready: true`, a rebuilt cache, every declared fixture, and at least one writeback candidate per writable calendar.

Do not treat a successful legacy-overlay build as Contract v1 proof. For this first beta, retain the Studio Stage and Windows installer proof with the candidate evidence. A missing or malformed contract declaration must fail closed; a release without the declaration is the only allowed legacy fallback.

## 3. Create the reviewed GitHub draft

Use the GitHub Publisher from its established fixed folder:

```powershell
Set-Location 'C:\Users\chris\Projects\Dash-Go_GitHub_Publisher_1.0.0'
.\Verify-DashGoPublisherKit.ps1
.\Publish-DashGoRelease.ps1 -Action Diagnose
.\Publish-DashGoRelease.ps1 -Action Preflight
.\Publish-DashGoRelease.ps1 -Action Guided
```

`Preflight` verifies the selected source handoff and Builder-produced public source asset agree. `Guided` creates the source commit, exact tag, and GitHub **draft** release after its explicit confirmations. It records a transaction journal before mutation so an interrupted local-only attempt can be diagnosed and recovered safely.

### GitHub release-note contract

The current version section at the top of `CHANGELOG.md` is the reviewed source of truth for the GitHub release body. The Publisher extracts only that exact version section, previews the final body before the typed draft confirmation, and verifies the initial GitHub draft body after creation.

The optional canonical `###` categories are:

- `Features`
- `Improvements`
- `Bug fixes`
- `Performance and reliability`
- `Security`
- `Build and release integrity`
- `Removals`
- `Upgrade notes`
- `Known issues`

Omit categories with no meaningful entries. `Removals` is reserved for significant user-visible, compatibility, platform, provider, setting, API, file-format, or supported-workflow removals. Do not use it for dead variables, internal helpers, duplicate tests, dependency cleanup, comments, unused styles, or other implementation housekeeping.

The source gate and Publisher both fail closed when the current version section is missing, not first, empty, contains unsupported or duplicate category headings, or has a category without at least one bullet. Stable promotion first consolidates the active beta history into one stable changelog section, so stable GitHub notes describe the complete stable delta rather than replaying each beta.

Review the draft before publication:

- version, tag, title, and beta/stable state;
- all four required assets, names, sizes, and GitHub-reported digests;
- local `SHA256SUMS` entries;
- release notes and source commit;
- absence of household data, credentials, private URLs, diagnostics, or personal screenshots.

## 4. Publish and verify the public route

After reviewing the draft:

```powershell
.\Publish-DashGoRelease.ps1 -Action Publish
.\Publish-DashGoRelease.ps1 -Action PublicProbe
```

`Publish` requires a typed confirmation. `PublicProbe` verifies the public GitHub Release metadata and assets used by normal installation and update discovery.

## Interrupted publication and recovery

Before beginning another release, run:

```powershell
.\Publish-DashGoRelease.ps1 -Action Diagnose
```

If the Publisher reports an unfinished transaction, resolve it before running `Preflight` or `Guided` again:

```powershell
.\Publish-DashGoRelease.ps1 -Action Recover
```

Recovery first creates a local snapshot, then may restore a proven Publisher-created local-only commit back to the recorded remote state. It never force-pushes, deletes remote data, runs `git clean`, or automatically resets unrelated commits. Draft-release continuation and public-release correction remain deliberate maintainer decisions.

## Public-release safety

- Release assets are public project material. Never publish household data, calendar URLs, credentials, backups, diagnostic exports, personal screenshots, or private hostnames.
- Keep GitHub tokens on the maintainer workstation only; the installed Dash-Go updater does not need one.
- Use the exact release tag for review and download checks. Do not rely on a moving latest selector for validation.
- A failed Builder or Publisher gate means no release: fix the source or workflow and rerun the normal validation path.

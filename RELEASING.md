# Releasing Dash-Go

Dash-Go is a public GitHub project. GitHub Releases are the canonical installer and update distribution channel. The release workflow is deliberately fail-closed: source is audited, the local builder creates every generated asset, GitHub publication starts as a draft, and the public route is probed only after publication.

The local Windows/WSL builder owns generated browser bundles, Linux binaries, release archives, SPDX SBOMs, checksums, and release catalogs. Do not hand-edit an archive, SBOM, `SHA256SUMS`, or catalog after a successful build.

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

## 1. Prepare the source repository

Start with the final source handoff, not a builder directory or a prior release-asset directory.

1. Extract the handoff into the dedicated Dash-Go Git working tree.
2. Confirm it contains no `AI.md`, generated binaries, browser bundles, release assets, logs, caches, backups, calendars, credentials, or local configuration.
3. Run `app/tests/public-source-repository-smoke.sh` from the source root.
4. Inspect `git status`, `git diff --cached`, `.gitignore`, and `.gitattributes` before committing.
5. Commit only the reviewed source tree to `DashDashGoApp/Dash-Go`.

## 2. Build release assets locally

Run the local builder from its established fixed folder:

```powershell
Set-Location 'C:\Users\chris\Projects\Calendar\Dash-Go_Local_Builder_1.0.3'
.\Build-DashGoRelease.ps1 -Performance Max
```

Select the exact source handoff and allow the Builder to complete every gate. On success it creates the release-asset directory used by the Publisher. Do not change files in that directory.

A same-version beta rebuild is allowed only before publication, only with the Builder’s explicit `-Force` confirmation, and never as a way to bypass a failed validation. Stable releases are never rebuilt or overwritten at the same version.

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

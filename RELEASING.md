# Releasing Dash-Go

This is the maintainer workflow for a Dash-Go GitHub Release. It intentionally separates a source commit, local release build, private rehearsal, and public publication.

The local Windows/WSL builder is the only tool that creates the four release assets. Do not hand-edit an archive, SBOM, or `SHA256SUMS` file after a successful build.

## Release asset set

Every release uses exactly these assets:

```text
Dash-Go_X.Y.Z[-beta.N]_release.tar.gz
Dash-Go_X.Y.Z[-beta.N]_source.tar.gz
Dash-Go_X.Y.Z[-beta.N]_sbom.spdx.json
SHA256SUMS
```

The exact tag is `vX.Y.Z` for stable or `vX.Y.Z-beta.N` for beta. A beta GitHub Release must be marked as a prerelease. A stable GitHub Release must not be marked as a prerelease.

## Private rehearsal

A private repository can be used to rehearse the source push, tag, draft release, asset upload, and authenticated maintainer download workflow. A published prerelease in a private repository remains private to authorized repository users.

Do not put a GitHub token on a Dash-Go VM or Pi. The installed updater is deliberately anonymous and cannot download assets from a private repository. Private rehearsal downloads happen on a maintainer workstation through authenticated GitHub access.

Before the repository is made public, decide whether its private rehearsal releases and tags should become visible. Remove any private-only draft/release and tag that should not be published before changing repository visibility. Never reuse a published immutable-release tag.

## Source push

Use a dedicated clean Git working tree. Start from the final source handoff, not an older source-import candidate, a builder folder, or a release-asset folder.

Before the first push:

1. Verify the source tree has no `AI.md`, `.git/`, release assets, generated binaries, logs, caches, backups, calendars, credentials, or local configuration.
2. Run `app/tests/public-source-repository-smoke.sh` from the source root, then inspect every staged file manually.
3. Inspect `git status` and `git diff --cached` before committing.
4. Confirm that `.gitignore` and `.gitattributes` are present.
5. Push only the audited source tree to `DashDashGoApp/Dash-Go`.

Use an annotated tag after the source commit is pushed:

```text
git tag -a vX.Y.Z-beta.N -m "Dash-Go X.Y.Z-beta.N"
git push origin vX.Y.Z-beta.N
```

## Draft, verify, then publish

1. Run the local builder and preserve the completed four-file asset directory unchanged.
2. Create a GitHub Release as a **draft** for the already-pushed exact tag.
3. Attach all four assets.
4. Confirm the title, beta/stable state, asset names, asset sizes, GitHub-reported digests, and local `SHA256SUMS` entries agree.
5. Save the draft for review or publish it as the intended private/public release.

When release immutability is enabled, draft releases can be reviewed and changed, but published releases lock their tag and assets. Use a new version for every correction after publication.

## Authenticated private-download rehearsal

On a maintainer workstation that has GitHub CLI access to the private repository:

```text
gh auth status
gh release download vX.Y.Z-beta.N --repo DashDashGoApp/Dash-Go --dir dash-go-download-test
audit the downloaded files against SHA256SUMS
```

Repeat the download only against the explicit tag. Do not use a moving `latest` selector for a release rehearsal.

## Public transition

Only make the repository public after the final privacy audit and source-history review. Before the first public prerelease:

- Enable the repository's intended immutable-release policy.
- Enable and verify the public private-vulnerability-reporting path.
- Confirm no private-only release, draft, tag, asset, commit, or documentation remains that should not become public.
- Publish a new public prerelease rather than modifying a private rehearsal release.

## Public-transition privacy rule

Before changing a private rehearsal repository to public, run the local
builder's private source-privacy audit against the exact staged handoff. If a
private rehearsal commit, tag, draft, release, or asset contains material that
must remain private, do not merely add a later corrective commit. Replace the
private rehearsal history and assets with a new clean repository history before
public visibility is enabled.

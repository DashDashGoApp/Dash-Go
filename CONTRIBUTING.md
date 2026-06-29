# Contributing to Dash-Go

Thank you for helping improve Dash-Go.

Dash-Go (pronounced **“Dash Dash Go”**) is a local-first touchscreen dashboard for shared households, designed primarily for Raspberry Pi kiosks, with special attention to the Raspberry Pi Zero 2 W. Contributions should preserve that focus: household usefulness, calm touch-first interaction, predictable local behavior, and bounded resource use.

## Before you start

- Search existing GitHub issues and discussions before opening a new one.
- Use public GitHub issues for bugs, documentation corrections, and feature proposals.
- Do not post security-sensitive details publicly. The supported private reporting path is stated in `SECURITY.md`; it must be enabled before the repository is made public.
- Do not include household data, calendar URLs, access tokens, API keys, passwords, notification routes, backups, or unredacted diagnostics in an issue, pull request, test fixture, or commit.

## What makes a good contribution

A good Dash-Go contribution:

- Solves a recurring household need or improves reliability, clarity, accessibility, or performance.
- Preserves useful offline and local-first behavior.
- Works through touch interactions as well as keyboard and mouse development workflows.
- Fits the existing Dash-Go visual language and theme system.
- Avoids unnecessary always-on timers, network work, large retained DOM trees, and memory-heavy behavior.
- Remains practical for the Raspberry Pi Zero 2 W Lite profile.
- Includes focused tests for changed behavior.

Before proposing a major new household app or provider integration, open a discussion describing the household problem, primary user flow, local/offline behavior, data ownership, resource impact, and correction path.

## Development model

Dash-Go source is maintained in GitHub. Official installers, source archives, and published releases are distributed through the official [Dash-Go GitHub repository](https://github.com/DashDashGoApp/Dash-Go) and GitHub Releases.

The project uses:

- Go for the dashboard control server.
- HTML, CSS, and JavaScript for the kiosk interface.
- Shell for installer, kiosk/session, maintenance, and operating-system integration.
- A local Windows/WSL release builder for release-package validation and GitHub Release asset creation.

The local builder is the authority for generated browser assets, compiled binaries, package validation, checksums, release metadata, and release archives.

Maintainers use [`RELEASING.md`](RELEASING.md) for the privacy-reviewed source push, exact tag, private rehearsal, draft-release, and public-release process. Release assets are never committed to the source repository.

## Source layout

```text
app/        Dash-Go application source, tests, and source release contract
installer/  Installer and repair behavior
```

`AI.md` is durable maintainer guidance supplied separately to assistants. It is not part of the public source tree, source handoff, release archive, or GitHub Release asset.

## Before opening a pull request

Keep each pull request focused on one understandable change. Separate unrelated cleanup, refactors, dependency updates, and user-facing behavior changes when practical.

Before submitting:

- Add or update focused tests that demonstrate the changed behavior.
- Use clear feature, domain, or outcome-based test names. Do not use beta or release numbers in ordinary test filenames.
- Run the relevant source checks available in your environment.
- Run Go formatting for any changed Go source.
- Test touch changes through Dash-Go’s real pointer-release path, not only synthetic browser clicks.
- Review visible interface changes at both 1920×1080 and 1024×600.
- Consider Lite-profile impact whenever a change adds timers, observers, animation, image work, layout measurement, network activity, retained state, or background processing.

For changes that affect releases, installers, generated assets, packaging, or published artifacts, maintainers run the local release builder before publication.

## Generated files and release artifacts

Do not commit or manually edit generated release artifacts unless a maintainer specifically requests it.

This includes:

```text
compiled server binaries
browser bundles and generated CSS
release tarballs and ZIP files
catalog manifests and checksums
local caches and logs
user configuration and calendar data
credentials, tokens, and private routes
```

Edit split source files instead. The release builder regenerates derived assets and validates the final package.

## Documentation

Update documentation when behavior changes:

- `README.md` for installation, ordinary use, and durable administrator guidance.
- `CHANGELOG.md` for published release history.
- `CURRENT_STATE.md` for immediate current-release status and active operational facts.
- `INTEGRATIONS.md` when an outside service is added, removed, or materially changed.
- `PRIVACY.md` when data storage, transmission, retention, or exposure changes.
- `THIRD_PARTY_NOTICES.md` when a distributed dependency or asset inventory changes.
- `SECURITY.md` when the project’s security reporting or supported-release policy changes.

Avoid adding temporary beta notes, personal scratchpad content, or stale implementation journals to user-facing documentation.

## Pull requests

A pull request should include:

- A concise explanation of the problem and solution.
- Screenshots or a short recording for visible UI changes, where practical.
- Tests added or updated.
- Documentation changes, when relevant.
- Any Lite-profile or Raspberry Pi Zero 2 W performance considerations.
- A note describing any migration, compatibility, or user-data effect.

Keep pull requests narrow. Separate unrelated cleanup, refactors, dependency upgrades, and user-facing behavior changes where practical.

## Coding and design expectations

- Keep Go domains cohesive and avoid cross-domain mutation through broad application objects.
- Keep browser modules split by responsibility.
- Preserve the shared overlay, app-launcher, OSK, theme, and touch-interaction contracts.
- Use existing theme variables and Dash-Go component patterns rather than introducing unrelated visual systems.
- Avoid browser-native alert, confirm, and prompt dialogs for normal kiosk actions.
- Keep external integrations opt-in and clearly distinguish local records from provider-backed records.
- Do not add background automation, provider syncing, or network polling without a bounded lifecycle and clear user value.

## Licensing

Dash-Go is licensed under the MIT License.

By submitting a contribution, you agree that your contribution may be distributed under the MIT License. You retain ownership of your contribution, subject to that license grant.

## Code of conduct

Be respectful, constructive, and mindful that Dash-Go is intended for household and family use. Harassment, discrimination, abuse, and publication of another person’s private information are not welcome.

## Questions and ideas

Use GitHub Discussions or a feature-request issue for design questions, household-workflow ideas, and early proposals.

For ordinary support, use the project’s public support channel. For security concerns, follow the current reporting instructions in `SECURITY.md`.

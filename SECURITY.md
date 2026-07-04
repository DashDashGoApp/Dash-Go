# Security Policy

## Reporting a vulnerability

When the repository Security page shows **Report a vulnerability**, use that
private GitHub reporting flow for security-sensitive details. Do not report
security-sensitive information through public GitHub issues, discussions, pull
requests, social media, or public chat.

If the private reporting control is not visible, do not disclose sensitive
details publicly. People who already have authorized repository access should
use the private maintainer channel through which that access was granted.

Do not include household data, calendar URLs, API keys, access tokens,
passwords, private notification routes, backup archives, or unredacted
diagnostics in any report.

## Supported releases

Security fixes are provided for the current stable Dash-Go release.

Beta releases may receive security corrections while actively being tested, but they are not treated as long-term supported releases. Users should move to the current stable release when practical.

## What to report

Examples of security concerns include:

- Remote or local code execution.
- Unauthorized access to Dashboard Control or household data.
- Authentication, authorization, PIN, or permission bypasses.
- Exposure of credentials, tokens, private calendar URLs, notification routes, or backups.
- Update, installer, package-verification, or release-integrity weaknesses.
- Network exposure beyond Dash-Go’s intended loopback-only dashboard service.
- Unsafe handling of user-controlled calendar, task, message, map, or integration data.
- Denial-of-service conditions that can materially compromise kiosk availability or device safety.

## Scope

Dash-Go is designed for locally administered household devices. Security reports may cover the Dash-Go source, installer, locally installed dashboard, release artifacts, official GitHub repository configuration, and official GitHub Release assets.

The following are normally outside Dash-Go’s direct security scope:

- Physical access to an unlocked Raspberry Pi or removable storage.
- A compromised operating system, Linux account, home network, router, or third-party provider account.
- Availability, privacy, security, rate limits, or terms of optional third-party services.
- Vulnerabilities in Raspberry Pi OS, browsers, Go, operating-system packages, or third-party services that do not arise from Dash-Go’s use or configuration of them.
- Reports requiring abnormal, destructive, or unsafe testing against another person’s device or account.

A report may still be useful when a third-party issue materially affects Dash-Go users. Include the relevant Dash-Go behavior and affected dependency or provider details.

## Good-faith research

Dash-Go welcomes good-faith security research that:

- Avoids privacy violations, service disruption, destructive actions, and access to data that does not belong to the researcher.
- Uses the minimum testing needed to demonstrate the issue.
- Protects household data and secrets.
- Gives maintainers a reasonable opportunity to investigate and prepare a fix before public disclosure.

Do not attempt to access, modify, delete, or exfiltrate data from systems or accounts that you do not own or have explicit permission to test.

## Fixes and disclosure

Maintainers will review private reports, request clarification when needed, and coordinate a fix or mitigation when the issue is confirmed.

When a fix is released, Dash-Go will publish the corrected version through the official GitHub Releases page. Where appropriate, the release notes or a GitHub Security Advisory will describe the affected versions, impact, mitigation, and credit for the reporter.

## Security practices for administrators

Dash-Go administrators should:

- Install releases only from the official Dash-Go GitHub repository and GitHub Releases.
- Keep Raspberry Pi OS and installed system packages updated.
- Use SSH keys or a strong account password.
- Keep the dashboard control server bound to loopback unless they fully understand the implications of changing its network exposure.
- Protect physical access to the device, storage, backups, and SSH credentials.
- Treat calendar URLs, API keys, notification routes, access tokens, and diagnostic exports as sensitive.
- Review backups before copying or storing them outside the device.

## Contact

Use the repository's **Report a vulnerability** flow when it is visible. Otherwise, use the authorized private maintainer channel and do not disclose sensitive details publicly.

For ordinary bugs, documentation corrections, feature requests, and support questions, use the appropriate public GitHub issue, discussion, or support channel.

## One-shot Google OAuth callback and kiosk QR

When a private Google Calendar connection is authorized with a Web OAuth client, Dash-Go exposes only a narrow one-shot callback route. The local owner’s authorization command first writes a random state value to an owner-only relay directory. The callback returns `404` unless that state is armed, unexpired, and an exact constant-time match; a successful callback records only the short-lived authorization code, removes the armed state, and then returns to `404`. The dashboard control server never receives the OAuth client secret and does not exchange or refresh tokens through this route. The CLI performs that exchange locally and writes the final token under the existing owner-only vdirsyncer home.

The associated dashboard QR overlay is created only by an administrator actively authorizing a named connection, uses the same short armed window, and disappears when the flow completes, times out, or is cancelled. It carries the Google authorization URL but no client secret or token. A household member who scans it can authorize the Google account they select for that named connection, so administrators should arm it only while present; use the Desktop paste-back flow or the helper’s `-no-display` switch when that enrollment risk is not acceptable. Do not publish Dash-Go’s general loopback control API to a LAN or the Internet. A Web OAuth callback must be an administrator-managed exact HTTPS reverse-proxy route with only this one callback exposed.

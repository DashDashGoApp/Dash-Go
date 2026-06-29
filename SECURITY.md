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

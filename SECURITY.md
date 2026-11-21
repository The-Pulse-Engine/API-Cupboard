# Security Policy

API Cupboard keeps API keys. Security reports have high priority.

## Supported versions

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Report a vulnerability

Do not open a public issue.

Use GitHub private vulnerability reporting:

1. Go to the **Security** tab of this repository.
2. Click **Report a vulnerability**.
3. Include:
   - The affected version and OS
   - The steps to reproduce the problem
   - The effect (for example, key exposure or bypass of a check)

Do not include real API keys in your report. Use test keys.

## What happens next

1. We confirm that we received your report.
2. We check the problem and tell you if we accept it.
3. We fix it, release a new version and credit you in the release notes, unless you ask us not to.

Please do not disclose the problem in public until a fix is released.

## Scope

**In scope:**

- Key exposure through logs, errors, the request log or the UI
- Bypass of the `Host` or `Origin` checks
- Bypass of the UI session token or the upstream URL confirm step
- Path traversal through shelf names
- Keys sent to a non-`https` upstream
- Access to the proxy or UI from another device on the network

**Out of scope:**

- Malware or other processes that run as your own user account. These can call the proxy and `cupboard run`. This is a known limit and is documented in [`docs/threat-model.md`](docs/threat-model.md).
- Attacks that need physical access to an unlocked machine
- Problems in the OS keychain itself
- Problems in upstream APIs

# Security Policy

## Supported Versions

TradeMind is currently in an early open-source MVP stage. Security fixes are prioritized for the `main` branch and the latest public release when releases are available.

| Version | Supported |
| --- | --- |
| `main` | Yes |
| Latest release | Yes |
| Older releases | Best effort |

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub Issues.

If you discover a vulnerability, please contact the maintainer privately:

- GitHub: <https://github.com/lien0219>

When reporting a vulnerability, please include:

- A clear description of the issue.
- Reproduction steps or proof of concept.
- Affected version, commit, or deployment mode.
- Potential impact.
- Suggested mitigation if available.

Please do not include real API keys, platform tokens, cookies, passwords, or other secrets in the report.

## Security Scope

TradeMind handles sensitive operational data, including:

- AI API keys
- Storage access keys
- Platform app secrets
- Store access tokens and refresh tokens
- Webhook secrets
- Admin account credentials

These values must not be committed to the repository. Use `.env` locally, keep `.env` out of git, and configure runtime secrets securely in production.

## Disclosure Process

We aim to acknowledge security reports as soon as possible and coordinate fixes responsibly. Public disclosure should happen only after a fix or mitigation is available.

## Production Deployment Notes

Before exposing TradeMind to a public network, you should:

- Change `JWT_SECRET`, `APP_MASTER_KEY`, admin bootstrap password, and database passwords.
- Use HTTPS and a trusted reverse proxy.
- Restrict database and Redis access to private networks.
- Avoid logging secrets, tokens, cookies, or complete third-party API responses.
- Review platform OAuth callback URLs and permissions.
- Back up PostgreSQL data and uploaded files.

# Security Policy

全局主密钥轮换仅限 tenant 0 的有效系统管理员。`security.key.rotate` 不可委派；普通租户管理员不能读取全局引用或轮换任务。轮换控制与审计同事务提交，审计不可用时拒绝变更。审计完整性查询只能校验认证租户自己的链。

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

Console accounts must be provisioned by an authorized administrator. Public registration and registration-code endpoints deny account creation. Sensitive RBAC, password, session-revocation and shop-credential changes must commit with their SQL audit evidence; audit failures return errors and roll back the associated local mutation. Tokens are bound to persisted security versions and, when applicable, server-side sessions. See `docs/RBAC_PERMISSION_DESIGN.md` and `docs/SECURITY_PERMISSION_CHECKLIST.md` for the permission matrix, verification limits and recovery boundaries.

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

## P10 Credential and Read-only Boundary

P10 stores Provider credential values only in backend AES-256-GCM envelopes whose AAD binds tenant, platform, credential ID, and version. Admin/API DTOs expose metadata only. Local keys and offline OAuth are development/test-only and fail closed in staging/production; a managed key provider is still required before external activation.

OAuth state is random, expiring, single-use, and tenant/user/platform/shop/redirect bound. Redirects use an exact configured allowlist. The P10 Douyin adapter accepts only the trusted official HTTPS host, applies connection/request/header timeouts and a response-size limit, maps errors to safe internal codes, and exposes no inventory write capability.

Current P10 runtime remains `L0`: real platform network, real credentials, real inventory read/write, inventory mutation, Worker, automatic business retry, Gray, and Production Ready are all disabled. Five kill switches take precedence over feature flags, and the write kill switch is permanently active in this release.

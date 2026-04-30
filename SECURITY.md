# Security Policy

## Supported Versions

| Version | Support Status |
|---------|----------------|
| v1.0.x  | Active — security patches for latest minor only |
| < v1.0  | Not supported — please upgrade |

Security fixes are applied only to the latest minor release. We do not backport patches to older versions.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Instead, use GitHub's private vulnerability reporting:

1. Go to the [Security Advisories page](https://github.com/rethink-paradigms/mesh/security/advisories)
2. Click **"New draft security advisory"**
3. Provide a clear description, reproduction steps, and impact assessment

This ensures responsible disclosure and gives us time to develop and release a fix before details become public.

## Security Update Process

1. **Acknowledgment** — We acknowledge receipt within 48 hours
2. **Assessment** — We triage severity and assign a CVE where applicable
3. **Patch Development** — Critical vulnerabilities: patch within 7 days. High: within 14 days. Medium/Low: next scheduled release
4. **Coordinated Disclosure** — We publish a security advisory and release the patch simultaneously. Reporters are credited (with consent)
5. **Notification** — Watch the repository's **Security** tab or enable GitHub security alerts to receive notifications

## Security Design Principles

Mesh is built with security-by-design principles that reflect its architecture:

- **User-Owned Compute (C3)**: You own all compute, keys, and network. Mesh does not operate a hosted platform, control your infrastructure, or hold your credentials
- **No Telemetry, No Central Dependency (C4)**: Mesh has no phone-home mechanism, no account system, and no central coordination server. There is no external attack surface from a Mesh-controlled service because none exists
- **Self-Hosted by Default**: Mesh runs on your infrastructure. Security boundaries are your boundaries
- **Minimal Attack Surface**: The core is intentionally small. Provider integrations are plugins, not core libraries, limiting the blast radius of any single vulnerability
- **Portable, Not Persistent**: Body snapshots are filesystem tarballs (`docker export | zstd`). No memory state is captured, reducing the risk of credential leakage through snapshots

## Scope

This policy covers the Mesh core repository (`rethink-paradigms/mesh`) and its official plugins. Third-party substrate providers or plugins maintained outside this organization are not in scope.

## Acknowledgments

We thank security researchers and community members who report vulnerabilities responsibly. Your efforts help keep Mesh users safe.

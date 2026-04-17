# Security Policy

## Supported Versions

| Version | Supported |
|:---|:---|
| 0.3.x | Yes |
| < 0.3 | No |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, report vulnerabilities using GitHub's private vulnerability reporting:

1. Go to [github.com/rethink-paradigms/mesh/security](https://github.com/rethink-paradigms/mesh/security)
2. Click **"Report a vulnerability"**
3. Fill in the advisory form

### What to Include

- **Description**: What the vulnerability is and its potential impact
- **Affected versions**: Which versions are affected
- **Reproduction steps**: How to trigger the vulnerability
- **Proof of concept**: Code or commands demonstrating the issue (if applicable)
- **Suggested fix**: If you have ideas for remediation

## Response Timeline

| Step | Expected Time |
|:---|:---|
| Acknowledgment | Within 48 hours |
| Initial assessment | Within 5 business days |
| Status updates | Weekly until resolution |
| Fix or mitigation | Depends on severity and complexity |

## Disclosure Policy

- Vulnerabilities are disclosed after a fix is released and users have had reasonable time to update.
- We coordinate disclosure timing with the reporter.
- CVEs are requested for significant vulnerabilities.
- We appreciate responsible disclosure and will credit reporters (unless they prefer to remain anonymous).

## Security Architecture

Mesh is designed with security-first principles:

- All mesh traffic is encrypted via WireGuard (Tailscale)
- External endpoints use TLS/HTTPS with automatic Let's Encrypt certificates
- Containers run with Docker isolation and resource limits
- Zero SSH access — all configuration is declarative

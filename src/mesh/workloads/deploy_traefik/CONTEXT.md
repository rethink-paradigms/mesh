# Feature: Deploy Traefik with TLS/HTTPS

**Description:**
Deploys Traefik as an ingress controller with automatic TLS certificate provisioning via Let's Encrypt ACME.

## 🧩 Interface

### Python API

**Function:** `deploy_traefik()`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `acme_email` | `str` | required | Email for Let's Encrypt notifications |
| `acme_ca_server` | `str` | `"letsencrypt"` | ACME server (letsencrypt, letsencrypt-staging) |
| `acme_tls_challenge` | `bool` | `True` | Use TLS challenge for certificate validation |
| `acme_http_challenge` | `bool` | `False` | Use HTTP challenge (fallback) |
| `memory` | `int` | `256` | Memory allocation in MB |
| `cpu` | `int` | `200` | CPU allocation in MHz |
| `nomad_addr` | `str` | `None` | Nomad server address |

**Returns:** `bool` - True if deployment succeeded

### Nomad Job Template

**File:** `traefik.nomad.hcl`

## 📦 Dependencies

- Nomad cluster running
- Consul for service discovery
- Public IP address or domain name
- Let's Encrypt rate limits (50 certs per domain per week)

## 🧪 Tests

- [ ] Test: Traefik job template validates
- [ ] Test: Traefik deploys successfully
- [ ] Test: ACME certificates are obtained
- [ ] Test: HTTP → HTTPS redirect works

## 📝 Example Usage

### Python API
```python
from src.workloads.deploy_traefik import deploy_traefik

deploy_traefik(
    acme_email="admin@example.com",
    acme_ca_server="letsencrypt-staging",  # Use staging for testing
    memory=256
)
```

### CLI
```bash
python src/workloads/deploy_traefik/deploy.py \
    --acme-email admin@example.com \
    --acme-ca-server letsencrypt-staging
```

### Nomad CLI
```bash
nomad job run \
  -var="acme_email=admin@example.com" \
  -var="acme_ca_server=letsencrypt-staging" \
  src/workloads/deploy_traefik/traefik.nomad.hcl
```

## 🔍 How It Works

1. **ACME Configuration**: Traefik configured with Let's Encrypt ACME
2. **Certificate Resolution**: Automatic certificate request on first HTTPS request
3. **Certificate Storage**: Stored in `/letsencrypt/acme.json` (persistent volume)
4. **HTTP Redirect**: All HTTP traffic redirected to HTTPS
5. **Service Discovery**: Consul integration for automatic backend discovery

## ⚙️ Configuration

### ACME Certificate Resolvers

**TLS Challenge** (Recommended):
- Port 443 must be accessible from internet
- Works behind most load balancers
- No port 80 required

**HTTP Challenge** (Fallback):
- Port 80 must be accessible from internet
- Required if TLS challenge fails
- More compatible with legacy networks

### Certificate Storage

- **Location**: `/letsencrypt/acme.json`
- **Persistence**: Host volume mount required
- **Backup**: Include in Consul/Nomad backup strategy

### Let's Encrypt Environments

| Environment | URL | Rate Limit |
|-------------|-----|------------|
| Production | `https://acme-v02.api.letsencrypt.org/directory` | 50 certs/week/domain |
| Staging | `https://acme-staging-v02.api.letsencrypt.org/directory` | Unlimited |

**Recommendation:** Always test with staging first!

## 🔒 Security Considerations

### ACME Email
- Required for Let's Encrypt expiration notices
- Use a monitored email address
- Consider using a mailing list for team notification

### Certificate Storage
- Store `acme.json` securely (contains private keys)
- Include in backup strategy
- Use host volume to persist across restarts

### HTTP Challenge Fallback
- Enable only if TLS challenge fails
- Requires opening port 80
- Use with caution in production

### Firewall Rules
- **TLS Challenge**: Allow TCP 443 from internet
- **HTTP Challenge**: Allow TCP 80 from internet
- **Consul**: Allow TCP 8500 from cluster only

## 📋 Service Integration

### Updating Web Services

To use HTTPS with Traefik, update your Nomad job tags:

**Before (HTTP only):**
```hcl
service {
  tags = [
    "traefik.enable=true",
    "traefik.http.routers.myapp.rule=Host(`myapp.example.com`)"
  ]
}
```

**After (HTTPS enabled):**
```hcl
service {
  tags = [
    "traefik.enable=true",
    "traefik.http.routers.myapp.rule=Host(`myapp.example.com`)",
    "traefik.http.routers.myapp.tls=true",
    "traefik.http.routers.myapp.tls.certresolver=letsencrypt"
  ]
}
```

### Automatic HTTP → HTTPS Redirect

Traefik automatically redirects HTTP to HTTPS when:
1. Router has `tls=true` tag
2. Router has certificate resolver configured
3. Middleware `redirect-http-to-https` is applied

## 🚨 Troubleshooting

### Certificates Not Issuing

**Symptoms:** HTTPS requests fail, certificate warnings

**Checks:**
1. Verify domain DNS points to Traefik public IP
2. Check ports 80/443 are accessible from internet
3. Review Traefik logs: `nomad logs traefik`
4. Check ACME rate limits: https://letsencrypt.org/docs/rate-limits/

**Solutions:**
- Use staging environment for testing
- Verify firewall rules allow inbound ports
- Check DNS propagation: `dig myapp.example.com`
- Wait for rate limit expiry (1 week)

### Certificate Storage Issues

**Symptoms:** Certificates lost on restart

**Solutions:**
- Mount host volume for `/letsencrypt`
- Include `acme.json` in backup strategy
- Verify volume permissions

### Consul Integration Issues

**Symptoms:** Services not registered in Traefik

**Checks:**
1. Verify Consul is accessible: `consul members`
2. Check service registration: `consul catalog services`
3. Review Traefik dashboard: http://traefik.example.com:8080

**Solutions:**
- Restart Traefik job: `nomad job restart traefik`
- Verify Consul token permissions
- Check service tags are correctly formatted

## 📚 References

- [Traefik ACME Documentation](https://doc.traefik.io/traefik/https/acme/)
- [Let's Encrypt Rate Limits](https://letsencrypt.org/docs/rate-limits/)
- [Nomad + Traefik Integration](https://www.nomadproject.io/docs/integrations/traefik)

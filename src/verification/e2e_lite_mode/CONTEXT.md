# Feature: E2E Lite Mode Tests

**Description:**
End-to-end tests for the lite mode HTTPS ingress system, validating single-VM deployments with Caddy.

## Prerequisites
- Multipass (for local testing) or AWS credentials (for cloud testing)
- Nomad cluster with tier=lite configuration
- Caddy deployed as lite ingress

## Test Scenarios

1. **Boot Verification**: Single node boots with Nomad + Caddy only (no Consul, no Tailscale)
2. **HTTPS Certificate**: Web app deployed with domain gets automatic HTTPS certificate
3. **Domain Routing**: Multiple apps with different domains route correctly
4. **Zero-Downtime Deploy**: Redeploying app doesn't drop connections
5. **Memory Budget**: Total overhead stays under 200MB

## Dependencies
- infrastructure/progressive_activation — TierConfig for tier detection
- workloads/deploy_lite_ingress — Caddy deployment and route management
- workloads/deploy_lite_web_service — Lite web service deployment

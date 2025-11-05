# MCP Auth Proxy Product Roadmap

## Product Vision & Positioning
- Deliver a "secure compatibility layer" for MCP clients that need to connect to legacy or non-standard upstream services. Emphasize turnkey authentication alignment with existing `auth-gateway` expectations, fast local deployment, and strong observability so platform teams can adopt it without rewriting upstreams.
- Highlight differentiators: pluggable signing strategies, managed secret rotation hooks, and production-grade telemetry. Position as the bridge for AI agent platforms (Claude, Codex, custom MCP clients) to reach enterprise backends safely.

## Strategic Roadmap

### Stabilize core proxy for GA-readiness (0–3 months)
1. Finalize configuration validation in `pkg/config` (strict env parsing, optional file loader) and document fallback order in `README.md`.
2. Introduce regression tests in `pkg/proxy` covering streaming edge cases (e.g., large payloads, upstream disconnect).
3. Ship binary packaging (Makefile, container image) under `cmd/auth-proxy` for reproducible deployment.
4. Publish quick-start guides in `docs/` with example MCP client configs for main agent ecosystems.

### Security & compliance enhancements (parallel 0–4 months)
1. Implement secret sourcing from cloud secret managers (AWS/GCP/Azure) via pluggable providers in `pkg/config/secrets`.
2. Add request signing policy enforcement (clock skew checks, nonce tracking) within `pkg/auth`.
3. Provide configurable audit logging and redactable fields in structured logs (`pkg/logging`).
4. Deliver security hardening documentation (threat model, hardening checklist) under `docs/security.md`.

### Observability & reliability expansion (2–5 months)
1. Add Prometheus/OpenTelemetry exporters in `pkg/metrics` with dashboards referenced in `deploy/`.
2. Introduce health/readiness endpoints in `cmd/auth-proxy` and synthetic integration tests in `test/integration/`.
3. Implement circuit breaking and retry policies with metrics-backed configuration in `pkg/proxy`.
4. Provide SLO playbook and on-call runbook under `docs/operations.md`.

### Ecosystem integrations & extensibility (4–8 months)
1. Create plugin interface for custom auth flows (e.g., OAuth token exchange) in `pkg/auth/plugins`.
2. Ship reference integrations: API Gateway (Lambda@Edge), Service Mesh (Istio mTLS), Zero Trust identity providers (Okta, Auth0) documented in `docs/integrations/`.
3. Offer Terraform/Helm modules in `deploy/` for cloud rollout and include CI templates in `.github/workflows/`.
4. Add SDK/examples for partner MCP clients, publishing sample repos under `examples/`.

### Commercial readiness & positioning (ongoing)
1. Develop comparison collateral vs. direct `auth-gateway` and open-source proxies in `docs/positioning.md`.
2. Define pricing/packaging levers (community vs. enterprise) with feature gating flags in `pkg/config/features`.
3. Launch customer feedback loop—usage analytics (privacy-conscious) and roadmap intake documented in `docs/customer-success.md`.
4. Prepare compliance roadmap (SOC2, ISO) with gap assessments tracked in `docs/compliance/`.

## Relevant Security Constructs
- Layered secret management, signing policy controls, runtime auditing, transport security (mTLS/TLS termination), and rate limiting/circuit breakers to contain upstream abuse.
- Integrate with enterprise IAM, SIEM, and policy enforcement points; ensure minimal secret exposure and comprehensive logging for forensics.

## Integration Touchpoints
- Cloud secret stores (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault), metrics backends (Prometheus/Grafana, Datadog), incident tooling (PagerDuty, Opsgenie), and MCP client SDKs.
- Deployment targets: Kubernetes (Helm), serverless proxies, edge gateways, enabling co-selling with AI agent platforms and observability vendors.


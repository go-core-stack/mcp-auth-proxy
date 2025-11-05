# MCP Auth Proxy – Implementation Plan

This document captures the initial plan for building a local MCP authentication proxy that mirrors the `auth-gateway` signing flow while accepting unsecured requests from standard MCP clients.

## 1. Requirements Clarification
- Define supported auth schemes (custom headers, HMAC, API key/secret), request verbs, streaming needs, and error semantics.
- Enumerate MCP client expectations (HTTP vs Unix socket, JSON-RPC limits) and unsupported flows (interactive OAuth).
- Decide on secrets source (environment variables vs secret manager) and logging redaction rules.

## 2. Architecture Overview
- Components: local MCP listener, proxy core, outbound auth injector, configuration loader, observability layer.
- Draft request flow diagrams covering success, missing auth, upstream errors, and retry scenarios.
- Adopt Go package layout with a top-level `main.go` entrypoint alongside `pkg/proxy`, `pkg/auth`, `pkg/config`, `pkg/metrics` mirroring `auth-gateway` conventions.

## 3. Configuration & Secrets
- Environment variables: `MCP_UPSTREAM_URL`, `MCP_API_KEY`, `MCP_API_SECRET`, `MCP_LISTEN_ADDR`, timeouts.
- Provide config struct with validation and optional YAML/JSON file merge; confirm how hot reload should work (explicit reload endpoint vs restart).
- Plan per-request overrides and secure logging that avoids printing secrets.

## 4. Local Server Scaffolding
- Use `net/http` to expose a local HTTP endpoint compatible with existing MCP agents; evaluate TLS termination as a follow-up.
- Implement canonical routes that accept MCP JSON-RPC payloads and forward them upstream.
- Configure connection pooling, sane read/write timeouts, and graceful shutdown hooks. **Status:** Implemented via `main.go` with graceful shutdown support.

## 5. Authentication Middleware
- Reproduce `auth-gateway` signature logic: timestamp, HMAC signature, and header injection. **Status:** Implemented in `pkg/auth/signer.go`.
- Support secret rotation by re-reading env vars periodically or on signal.
- Propagate request IDs through logging and upstream headers for traceability. **Status:** Logging uses structured fields; request ID support remains a future enhancement.

## 6. Proxy Core
- Build reverse-proxy handler via `httputil.ReverseProxy` or custom transport for header control. **Status:** Implemented with bespoke handler in `pkg/proxy/proxy.go`.
- Sanitize inbound headers, replay request bodies safely, and preserve streaming responses.
- Map upstream failures to local error responses and apply retries/backoff for transient issues. **Status:** Upstream error bodies are logged and forwarded; retry logic is still pending.
- Provide SSE fallback so `GET /mcp` remains responsive even when the upstream lacks streaming support. **Status:** Implemented with heartbeat keepalive.
- Handle `.well-known/oauth-authorization-server` discovery probes locally to avoid upstream 404 noise. **Status:** Implemented.

## 7. Observability & Diagnostics
- Structured logging (JSON) with request ID, method, status code, latency fields. **Status:** JSON logs with method/path/status are emitted; request IDs remain future work.
- Metrics instrumentation (Prometheus) for request counts, latency, error rates; optionally add OpenTelemetry hooks.
- Health endpoints `/healthz` and `/readyz` checking upstream reachability and config validity; include debug toggle.

## 8. Testing Strategy
- Unit tests: config parsing, signer correctness (known vectors), proxy handler behaviour (table-driven). **Status:** Implemented for signer and proxy (including SSE/discovery/error paths).
- Integration tests with mocked upstream verifying header injection, retries, timeout enforcement.
- Contract tests against a real `auth-gateway`-like service once credentials are available.

## 9. Tooling & CI
- Provide `make` targets for linting, testing, and building binaries/images; ensure `go test ./...` is CI baseline. **Status:** Go module with unit tests is in place; Makefile/C I automation still to be added.
- Supply Dockerfile and optional compose file for local orchestration with agents.
- Gate merges on lint/test workflows and document how to run them locally. **Status:** README documents the test command.

## 10. Agent Integration & Documentation
- Write setup guide describing env vars, launch command, and example `mcp.json` client config. **Status:** README documents required/optional env vars and workflow.
- Include troubleshooting section for common auth failures or upstream connectivity issues.
- Maintain change log for auth scheme adjustments to keep agents in sync. **Status:** Outstanding.

## 11. Rollout & Future Enhancements
- Plan deployment stages (local dev → staging → production) with rollback strategy.
- Track future features: TLS termination, multi-tenant key management, token refresh flow, caching, rate limiting.
- Review plan periodically as requirements evolve and update this document accordingly.

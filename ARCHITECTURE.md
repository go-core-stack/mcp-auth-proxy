# MCP Auth Proxy – Architecture Overview

The MCP Auth Proxy provides a local drop-in endpoint for MCP-aware agents that connects them to upstream MCP servers requiring the `auth-gateway` signing flow. This document captures the core components, their responsibilities, and the runtime communication paths between the MCP client/agent, the proxy, and the upstream MCP server.

## Component Topology

```
+-----------------+          +--------------------------------------+
| MCP Client /    |  JSON    |          MCP Auth Proxy              |
| Agent           |<-------->|  (Local HTTP listener + middleware)  |
+-----------------+          |                                      |
                             |  +-------------------------------+   |
                             |  | Local Listener (HTTP/JSON-RPC)|   |
                             |  +-------------------------------+   |
                             |  | Request Decoder               |   |
                             |  +---------------+---------------+   |
                             |                  |                   |
                             |  +---------------v---------------+   |
                             |  | Proxy Core / Reverse Proxy    |   |
                             |  +---------------+---------------+   |
                             |                  |                   |
                             |  +---------------v---------------+   |
                             |  | Auth Header Injector          |   |
                             |  +---------------+---------------+   |
                             |                  |                   |
                             |  +---------------v---------------+   |
                             |  | Observability & Logging       |   |
                             |  +---------------+---------------+   |
                             |                  |                   |
                             |  +---------------v---------------+   |
                             |  | Config & Secret Loader        |   |
                             |  +-------------------------------+   |
                             +--------------------------------------+
                                               |
                                               | HTTPS (signed JSON-RPC / REST)
                                               v
                                +-------------------------------+
                                | Upstream MCP Server           |
                                | (auth-gateway compatible API) |
                                +-------------------------------+
```

### Responsibilities
- **MCP Client / Agent** – Issues JSON-RPC invocations without embedded auth headers; expects transparent proxying and consistent responses.
- **Local Listener** – Exposes a `net/http` server on the configured listen address, performs basic request validation, and handles graceful shutdown.
- **Proxy Core** – Normalizes inbound requests, strips hop-by-hop headers, and streams responses back to the client using a tuned `http.Client`. Retries are deferred to the caller; the proxy performs a single upstream attempt per request.
- **Auth Header Injector** – Implements the `auth-gateway` style signing using method, path, and timestamp to derive an HMAC signature (`x-api-key-id`, `x-signature`, `x-timestamp`). Secrets are read at startup and remain static until the process restarts.
- **Observability & Logging** – Emits structured JSON logs (level, duration, upstream status, truncated error bodies). Metrics and health endpoints are future enhancements.
- **Config & Secret Loader** – Reads and validates environment variables (`MCP_UPSTREAM_URL`, `MCP_API_KEY`, `MCP_API_SECRET`, etc.) once during startup; dynamic reloads are not yet supported.
- **Local Fallbacks** – Serves a lightweight Server-Sent Events heartbeat on `GET /mcp` when the upstream lacks streaming support and short-circuits OAuth discovery probes with local 404s.
- **Upstream MCP Server** – Validates the signed requests, processes JSON-RPC payloads, and returns responses/errors that the proxy relays downstream.

## Request Lifecycle

```
Client/Agent          MCP Auth Proxy                       Upstream MCP Server
     |                        |                                        |
     | 1. JSON-RPC POST       |                                        |
     |----------------------->|                                        |
     |                        | 2. Decode and validate payload         |
     |                        |----------------------------------------|
     |                        | 3. Read cached env-derived config      |
     |                        |----------------------------------------|
     |                        | 4. Generate signature headers          |
     |                        |----------------------------------------|
     |                        | 5. Forward signed request  ----------->|
     |                        |                                        | 6. Verify signature & execute
     |                        |                                        |    upstream method
     |                        |<- - - - - - - - - - - - - - - - - - - -|
     |                        | 7. Stream response back to client      |
     | 8. Receive response    |                                        |
     |<-----------------------|                                        |
```

### Error and Retry Handling
- Missing or invalid secrets fail fast during startup; configuration issues are surfaced via fatal log entries with no proxy listener.
- Transient upstream failures (timeouts, 5xx) return their status and body directly to the caller. The proxy does not automatically retry requests.
- Authentication failures from upstream (401/403) are propagated intact, with contextual logging to aid operators.
- SSE heartbeat handling and discovery short-circuits never reach the upstream, eliminating noisy warning logs.

## Deployment Considerations
- **Secrets** – Sourced from environment variables (`MCP_API_KEY`, `MCP_API_SECRET`). Integrating with external secret managers will require extending the loader.
- **Transport Security** – Local listener starts HTTP-only for agent compatibility; TLS termination can be layered via a local reverse proxy when required.
- **Scalability** – Tailored for workstation or single-node deployment. Observability currently relies on logs; metrics/health endpoints can be layered in later iterations.
- **Extensibility** – Additional auth schemes (API tokens, mTLS) or dynamic config refresh can be added by extending the signer and loader interfaces.

This architecture ensures MCP clients can communicate with secured MCP servers without altering client code, while maintaining observability, secure secret handling, and compatibility with the `auth-gateway` authorization model.

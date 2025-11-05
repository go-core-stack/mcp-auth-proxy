# mcp-auth-proxy
Auth Proxy for handling authentication with non compatible mcp servers.

## Features
- Injects HMAC headers (`x-api-key-id`, `x-signature`, `x-timestamp`) compatible with the upstream auth-gateway implementation.
- Supports optional static session headers (`MCP_SESSION_HEADER`, `MCP_SESSION_VALUE`) so upstreams that expect pre-issued session IDs continue to work.
- Provides a local Server-Sent Events (SSE) keepalive endpoint for `GET /mcp` when the upstream does not offer streaming, allowing MCP clients (Codex, Claude, etc.) to complete their handshake.
- Short-circuits OAuth discovery probes (`/.well-known/oauth-authorization-server`) with local 404s to avoid noisy upstream errors.
- Structured JSON logging, including upstream error bodies (truncated to 64 KiB) for easier debugging.
- Table-driven unit tests covering signer behaviour, proxy forwarding, SSE fallback, discovery handling, and error propagation.

## Documentation
- See `IMPLEMENTATION_PLAN.md` for the current implementation roadmap.
- See `AGENTS.md` for coding conventions and commit guidelines.

## Usage
Set the required environment variables, then launch the proxy:

```bash
export MCP_UPSTREAM_URL="https://remote-mcp.example.com"
export MCP_API_KEY="your-api-key"
export MCP_API_SECRET="your-api-secret"
# optional static session header if upstream requires it:
# export MCP_SESSION_HEADER="x-session-id"
# export MCP_SESSION_VALUE="session-token"
# optional overrides:
# export MCP_LISTEN_ADDR="127.0.0.1:8080"
# export MCP_REQUEST_TIMEOUT="20s"

go run .
```

Point your local MCP-capable agent (Codex, Claude, etc.) at `http://127.0.0.1:8080`; the proxy will sign requests with HMAC headers before forwarding them to the upstream service.

## Testing
Run the full suite with:

```bash
GOCACHE=$(pwd)/.cache go test ./...
```

The tests exercise the signer, configuration, and proxy layers (including SSE fallback and error propagation) without requiring live network access.

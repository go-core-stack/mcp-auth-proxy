// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

// Package proxy provides an HTTP reverse proxy that bridges MCP agents with
// remote MCP servers. It injects the authentication headers mandated by the
// reference auth-gateway implementation, proxies JSON-RPC traffic, and offers
// optional conveniences such as serving a local event stream when the upstream
// lacks streaming support.
package proxy

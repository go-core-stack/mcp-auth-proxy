// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

// Package proxy contains the HTTP reverse proxy that fronts MCP clients while
// enriching outbound requests with authentication and session context expected
// by secure upstream servers.
package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-core-stack/mcp-auth-proxy/pkg/auth"
	"github.com/go-core-stack/mcp-auth-proxy/pkg/config"
)

// hopHeaders lists standard hop-by-hop headers that must be stripped before a
// request is proxied so the upstream connection semantics remain correct.
var hopHeaders = map[string]struct{}{
	"Connection":          {},
	"Proxy-Connection":    {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// Proxy forwards local MCP requests to a remote HTTP endpoint and injects auth
// headers while optionally handling discovery and event stream fallbacks.
type Proxy struct {
	// cfg keeps runtime knobs such as the upstream URL and shared secrets.
	cfg config.Config
	// client performs outbound HTTP requests with tuned transport settings.
	client *http.Client
	// signer injects HMAC headers compatible with the upstream auth gateway.
	signer *auth.Signer
	// logger emits structured logs for observability.
	logger zerolog.Logger
	// baseURL is the parsed upstream address used to resolve inbound paths.
	baseURL *url.URL
}

// New constructs a Proxy backed by an http.Client configured with sensible
// connection pooling defaults and the provided runtime configuration.
func New(cfg config.Config) (http.Handler, error) {
	// Build a transport that honours system proxies and keeps connections warm.
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, // nolint:gosec -- opt-in for development scenarios
		},
	}

	client := &http.Client{
		Timeout:   cfg.RequestTimeout,
		Transport: transport,
	}

	signer := auth.NewSigner(cfg.APIKey, cfg.APISecret)

	handler := &Proxy{
		cfg:     cfg,
		client:  client,
		signer:  signer,
		logger:  log.With().Str("component", "proxy").Logger(),
		baseURL: cloneURL(cfg.Upstream),
	}

	return handler, nil
}

// ServeHTTP applies protocol-specific shortcuts (SSE fallback, discovery
// responses) and otherwise streams the request/response pair to the upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	event := p.logger.With().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Serve a local keep-alive stream when Codex expects SSE but the upstream
	// does not expose one.
	if r.Method == http.MethodGet && isEventStreamPath(r.URL.Path) {
		p.serveEventStream(w, r, event)
		return
	}

	// Respond locally for discovery metadata probes to avoid noisy upstream 404s.
	if r.Method == http.MethodGet && isDiscoveryPath(r.URL.Path) {
		p.serveDiscovery(w, r, event)
		return
	}

	resp, err := p.forwardRequest(r, event)
	if err != nil {
		status := http.StatusBadGateway
		var httpErr *httpError
		if errors.As(err, &httpErr) {
			status = httpErr.Status
		}
		http.Error(w, http.StatusText(status), status)
		event.Error().
			Err(err).
			Dur("duration", time.Since(start)).
			Msg("request failed")
		return
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			event.Error().
				Err(closeErr).
				Msg("close upstream response body failed")
		}
	}()

	// Default to streaming the upstream body unless we need to inspect errors.
	var bodyReader io.Reader = resp.Body
	if resp.StatusCode >= http.StatusBadRequest {
		const maxLogBody = 64 * 1024 // limit to a manageable payload for logs.
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLogBody))
		if readErr != nil {
			event.Error().
				Err(readErr).
				Int("status", resp.StatusCode).
				Msg("failed to read upstream error body")
		} else {
			event.Warn().
				Int("status", resp.StatusCode).
				Bytes("upstream_body", payload).
				Msg("upstream returned error")
			bodyReader = bytes.NewReader(payload)
		}
	}

	cleanHopHeaders(resp.Header)
	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if _, copyErr := io.Copy(w, bodyReader); copyErr != nil {
		event.Error().
			Err(copyErr).
			Dur("duration", time.Since(start)).
			Msg("stream response failed")
		return
	}

	event.Info().
		Dur("duration", time.Since(start)).
		Msg("request proxied")
}

// forwardRequest clones the inbound request, augments headers, signs it, and
// returns the upstream response for the caller to stream back.
func (p *Proxy) forwardRequest(r *http.Request, event zerolog.Logger) (*http.Response, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			event.Error().
				Err(err).
				Msg("close request body failed")
		}
	}()

	targetURL := p.singleJoiningURL(r.URL)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	copyHeaders(upstreamReq.Header, r.Header)
	cleanHopHeaders(upstreamReq.Header)
	augmentForwardHeaders(upstreamReq.Header, r)

	if p.cfg.SessionValue != "" {
		// Attach the session header so the upstream can associate the call with an authenticated user.
		upstreamReq.Header.Set(p.cfg.SessionHeader, p.cfg.SessionValue)
	}

	upstreamReq.Host = targetURL.Host

	if err := p.signer.AttachSignature(upstreamReq); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	resp, err := p.client.Do(upstreamReq)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil, &httpError{Status: http.StatusGatewayTimeout, Err: err}
		default:
			var netErr net.Error
			if errors.As(err, &netErr); netErr != nil && netErr.Timeout() {
				return nil, &httpError{Status: http.StatusGatewayTimeout, Err: err}
			}
		}
		return nil, fmt.Errorf("perform upstream request: %w", err)
	}

	return resp, nil
}

// serveEventStream returns a minimal text/event-stream response with periodic
// keep-alive messages so MCP clients can complete their handshake.
func (p *Proxy) serveEventStream(w http.ResponseWriter, r *http.Request, event zerolog.Logger) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		event.Error().Msg("response writer does not support flushing for SSE")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if _, err := io.WriteString(w, ":ok\n\n"); err != nil {
		event.Error().Err(err).Msg("failed to send initial SSE comment")
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	event.Info().Msg("event stream opened")

	for {
		select {
		case <-r.Context().Done():
			event.Info().Msg("event stream closed")
			return
		case <-ticker.C:
			// Send a comment line which acts as a heartbeat for SSE clients.
			if _, err := io.WriteString(w, ":keepalive\n\n"); err != nil {
				event.Error().Err(err).Msg("failed to write keepalive")
				return
			}
			flusher.Flush()
		}
	}
}

// serveDiscovery currently returns a 404 placeholder so OIDC discovery probes
// do not propagate confusing upstream errors.
func (p *Proxy) serveDiscovery(w http.ResponseWriter, r *http.Request, event zerolog.Logger) {
	http.NotFound(w, r)
	event.Debug().Msg("discovery metadata not available; returning 404")
}

// isEventStreamPath checks for the canonical MCP GET endpoint used for SSE.
func isEventStreamPath(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	if trimmed == "" {
		return false
	}
	return trimmed == "/mcp"
}

// isDiscoveryPath identifies well-known OAuth discovery URL probes.
func isDiscoveryPath(path string) bool {
	return strings.HasPrefix(path, "/.well-known/oauth-authorization-server")
}

// singleJoiningURL resolves the incoming path relative to the configured base.
func (p *Proxy) singleJoiningURL(requestURL *url.URL) *url.URL {
	ref := &url.URL{
		Path:     requestURL.Path,
		RawPath:  requestURL.RawPath,
		RawQuery: requestURL.RawQuery,
		Fragment: requestURL.Fragment,
	}
	target := p.baseURL.ResolveReference(ref)
	return target
}

// cloneURL makes a shallow copy of the provided URL pointer.
func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	clone := *u
	return &clone
}

// copyHeaders appends all headers from src into dst.
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// cleanHopHeaders removes hop-by-hop headers that should not be forwarded.
func cleanHopHeaders(h http.Header) {
	for k := range hopHeaders {
		h.Del(k)
	}
}

// augmentForwardHeaders ensures X-Forwarded-* headers capture client metadata.
func augmentForwardHeaders(h http.Header, r *http.Request) {
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		prior := r.Header.Get("X-Forwarded-For")
		if prior != "" {
			clientIP = prior + ", " + clientIP
		}
		h.Set("X-Forwarded-For", clientIP)
	}
	if scheme := r.Header.Get("X-Forwarded-Proto"); scheme != "" {
		h.Set("X-Forwarded-Proto", scheme)
	} else {
		h.Set("X-Forwarded-Proto", "http")
	}
	h.Set("X-Forwarded-Host", r.Host)
}

// copyResponseHeaders mirrors headers from the upstream response to the writer.
func copyResponseHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// httpError wraps a status code with the underlying error from the upstream round trip.
type httpError struct {
	Status int   // Status preserves the HTTP status to emit downstream.
	Err    error // Err retains the original cause for logging.
}

// Error implements the error interface for httpError.
func (e *httpError) Error() string {
	return fmt.Sprintf("status %d: %v", e.Status, e.Err)
}

// Unwrap exposes the underlying error for errors.Is / errors.As checks.
func (e *httpError) Unwrap() error {
	return e.Err
}

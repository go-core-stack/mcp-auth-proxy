// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

package proxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-core-stack/mcp-auth-proxy/pkg/auth"
	"github.com/go-core-stack/mcp-auth-proxy/pkg/config"
)

func TestProxyForwardsAndSignsRequests(t *testing.T) {
	var (
		receivedMethod string
		receivedPath   string
		receivedBody   []byte
		receivedHeader http.Header
	)

	upstreamURL, err := url.Parse("https://upstream.example.com/root")
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	cfg := config.Config{
		ListenAddr:              "127.0.0.1:0",
		Upstream:                upstreamURL,
		APIKey:                  "key-id",
		APISecret:               "secret-value",
		SessionHeader:           "x-session-id",
		SessionValue:            "session-123",
		RequestTimeout:          time.Second,
		InsecureSkipVerify:      true,
		LogLevel:                "info",
		ServerReadTimeout:       time.Second,
		ServerWriteTimeout:      time.Second,
		ServerIdleTimeout:       time.Second,
		GracefulShutdownTimeout: time.Second,
	}

	handler, err := New(cfg)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	p, ok := handler.(*Proxy)
	if !ok {
		t.Fatalf("expected *Proxy, got %T", handler)
	}

	// Fix the timestamp so the signature can be asserted.
	fixedNow := time.Unix(1700000000, 0).UTC()
	p.signer.Now = func() time.Time { return fixedNow }

	p.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := req.Body.Close(); err != nil {
			return nil, err
		}
		receivedMethod = req.Method
		receivedPath = req.URL.Path
		receivedBody = body
		receivedHeader = req.Header.Clone()

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("upstream-ok")),
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "http://proxy/mcp", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if body := rec.Body.String(); body != "upstream-ok" {
		t.Fatalf("unexpected response body: %s", body)
	}
	if receivedMethod != http.MethodPost {
		t.Fatalf("expected method POST, got %s", receivedMethod)
	}
	if receivedPath != "/mcp" {
		t.Fatalf("expected upstream path /mcp, got %s", receivedPath)
	}
	if string(receivedBody) != `{"hello":"world"}` {
		t.Fatalf("unexpected upstream body: %s", string(receivedBody))
	}
	if got := receivedHeader.Get(cfg.SessionHeader); got != cfg.SessionValue {
		t.Fatalf("missing session header, got %q", got)
	}
	if got := receivedHeader.Get(auth.HeaderAPIKey); got != cfg.APIKey {
		t.Fatalf("missing api key header, got %q", got)
	}
	if ts := receivedHeader.Get(auth.HeaderTimestamp); ts != fixedNow.Format(time.RFC3339) {
		t.Fatalf("timestamp header mismatch: %q", ts)
	}

	expectedSig := computeSignature(cfg.APISecret, http.MethodPost, "/mcp", fixedNow.Format(time.RFC3339))
	if got := receivedHeader.Get(auth.HeaderSignature); got != expectedSig {
		t.Fatalf("signature mismatch: got %s want %s", got, expectedSig)
	}
}

func TestProxyServeEventStreamFallback(t *testing.T) {
	var outboundCalls int32

	upstreamURL, err := url.Parse("https://upstream.example.com")
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	cfg := config.Config{
		ListenAddr:              "127.0.0.1:0",
		Upstream:                upstreamURL,
		APIKey:                  "key-id",
		APISecret:               "secret-value",
		RequestTimeout:          time.Second,
		InsecureSkipVerify:      true,
		LogLevel:                "info",
		ServerReadTimeout:       time.Second,
		ServerWriteTimeout:      time.Second,
		ServerIdleTimeout:       time.Second,
		GracefulShutdownTimeout: time.Second,
	}

	handler, err := New(cfg)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	p, ok := handler.(*Proxy)
	if !ok {
		t.Fatalf("expected *Proxy, got %T", handler)
	}

	p.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&outboundCalls, 1)
		return nil, errors.New("should not call upstream for SSE fallback")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "http://proxy/mcp", nil).WithContext(ctx)
	rec := newFlushRecorder()

	done := make(chan struct{})
	go func() {
		p.ServeHTTP(rec, req)
		close(done)
	}()

	waitUntil(t, 500*time.Millisecond, func() bool {
		return strings.Contains(rec.body.String(), ":ok")
	})

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("event stream handler did not exit after context cancel")
	}

	if got := rec.header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("unexpected content type: %s", got)
	}
	if rec.status != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.status)
	}
	if !strings.Contains(rec.body.String(), ":ok") {
		t.Fatalf("expected initial SSE comment, got %q", rec.body.String())
	}
	if atomic.LoadInt32(&outboundCalls) != 0 {
		t.Fatalf("expected no outbound calls, got %d", outboundCalls)
	}
}

func TestProxyDiscoveryReturnsNotFound(t *testing.T) {
	var outboundCalls int32

	upstreamURL, err := url.Parse("https://upstream.example.com")
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	cfg := config.Config{
		ListenAddr:              "127.0.0.1:0",
		Upstream:                upstreamURL,
		APIKey:                  "key-id",
		APISecret:               "secret-value",
		RequestTimeout:          time.Second,
		InsecureSkipVerify:      true,
		LogLevel:                "info",
		ServerReadTimeout:       time.Second,
		ServerWriteTimeout:      time.Second,
		ServerIdleTimeout:       time.Second,
		GracefulShutdownTimeout: time.Second,
	}

	handler, err := New(cfg)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	p, ok := handler.(*Proxy)
	if !ok {
		t.Fatalf("expected *Proxy, got %T", handler)
	}

	p.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&outboundCalls, 1)
		return nil, errors.New("discovery fallback should not reach upstream")
	})

	req := httptest.NewRequest(http.MethodGet, "http://proxy/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if atomic.LoadInt32(&outboundCalls) != 0 {
		t.Fatalf("expected no outbound calls, got %d", outboundCalls)
	}
}

func TestProxyPropagatesErrorBodies(t *testing.T) {
	upstreamURL, err := url.Parse("https://upstream.example.com")
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	cfg := config.Config{
		ListenAddr:              "127.0.0.1:0",
		Upstream:                upstreamURL,
		APIKey:                  "key-id",
		APISecret:               "secret-value",
		RequestTimeout:          time.Second,
		InsecureSkipVerify:      true,
		LogLevel:                "info",
		ServerReadTimeout:       time.Second,
		ServerWriteTimeout:      time.Second,
		ServerIdleTimeout:       time.Second,
		GracefulShutdownTimeout: time.Second,
	}

	handler, err := New(cfg)
	if err != nil {
		t.Fatalf("create proxy: %v", err)
	}
	p, ok := handler.(*Proxy)
	if !ok {
		t.Fatalf("expected *Proxy, got %T", handler)
	}

	p.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("upstream-error")),
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "http://proxy/mcp", strings.NewReader("body"))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if body := rec.Body.String(); body != "upstream-error" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func computeSignature(secret, method, path, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	payload := strings.Join([]string{method, path, timestamp}, "\n")
	if _, err := mac.Write([]byte(payload)); err != nil {
		panic(fmt.Sprintf("write signature payload: %v", err))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

type flushRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{
		header: make(http.Header),
	}
}

func (r *flushRecorder) Header() http.Header {
	return r.header
}

func (r *flushRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *flushRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *flushRecorder) Flush() {}

func waitUntil(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

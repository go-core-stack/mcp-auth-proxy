// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

package auth

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestSignerAttachSignature(t *testing.T) {
	u, err := url.Parse("https://example.com/v1/test?foo=bar")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	req := &http.Request{
		Method: "POST",
		URL:    u,
		Header: make(http.Header),
	}

	signer := NewSigner("key123", "secret456")
	signer.Now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	err = signer.AttachSignature(req)
	if err != nil {
		t.Fatalf("AttachSignature: %v", err)
	}

	got := map[string]string{
		HeaderAPIKey:    req.Header.Get(HeaderAPIKey),
		HeaderSignature: req.Header.Get(HeaderSignature),
		HeaderTimestamp: req.Header.Get(HeaderTimestamp),
	}

	want := map[string]string{
		HeaderAPIKey:    "key123",
		HeaderSignature: "d1bfbc31386e7c029a0c30216ff01f5ed337b6de4bb97ac539c7a7feec125d05",
		HeaderTimestamp: "2023-11-14T22:13:20Z",
	}

	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s header mismatch: got %q, want %q", k, got[k], v)
		}
	}
}

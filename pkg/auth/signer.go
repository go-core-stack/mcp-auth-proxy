// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	HeaderAPIKey    = "x-api-key-id"
	HeaderSignature = "x-signature"
	HeaderTimestamp = "x-timestamp"
)

// Signer injects HMAC auth headers compatible with the upstream gateway.
type Signer struct {
	Key    string
	Secret string
	Now    func() time.Time
}

// NewSigner constructs a signer with the provided key/secret and sane defaults.
func NewSigner(key, secret string) *Signer {
	return &Signer{
		Key:    key,
		Secret: secret,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// AttachSignature mutates the request by injecting auth headers computed from the method,
// target path, and timestamp.
func (s *Signer) AttachSignature(req *http.Request) error {
	if s.Key == "" || s.Secret == "" {
		return fmt.Errorf("signer key and secret must be set")
	}

	timestamp := s.Now().Format(time.RFC3339)

	payload := strings.Join([]string{
		req.Method,
		req.URL.Path,
		timestamp,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(s.Secret))
	if _, err := mac.Write([]byte(payload)); err != nil {
		return fmt.Errorf("compute signature: %w", err)
	}

	sigBytes := mac.Sum(nil)
	signature := hex.EncodeToString(sigBytes)

	req.Header.Set(HeaderAPIKey, s.Key)
	req.Header.Set(HeaderSignature, signature)
	req.Header.Set(HeaderTimestamp, timestamp)

	return nil
}

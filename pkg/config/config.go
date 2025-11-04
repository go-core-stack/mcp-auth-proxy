// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envListenAddr             = "MCP_LISTEN_ADDR"
	envUpstreamURL            = "MCP_UPSTREAM_URL"
	envAPIKey                 = "MCP_API_KEY"
	envAPISecret              = "MCP_API_SECRET"
	envSessionHeader          = "MCP_SESSION_HEADER"
	envSessionValue           = "MCP_SESSION_VALUE"
	envRequestTimeout         = "MCP_REQUEST_TIMEOUT"
	envInsecureSkipVerify     = "MCP_UPSTREAM_INSECURE"
	envLogLevel               = "MCP_LOG_LEVEL"
	envServerReadTimeout      = "MCP_SERVER_READ_TIMEOUT"
	envServerWriteTimeout     = "MCP_SERVER_WRITE_TIMEOUT"
	envServerIdleTimeout      = "MCP_SERVER_IDLE_TIMEOUT"
	envGracefulShutdown       = "MCP_GRACEFUL_SHUTDOWN"
	defaultListenAddr         = "127.0.0.1:8080"
	defaultRequestTimeout     = 15 * time.Second
	defaultSessionHeader      = "x-session-id"
	defaultLogLevel           = "info"
	defaultServerReadTimeout  = 30 * time.Second
	defaultServerWriteTimeout = 30 * time.Second
	defaultServerIdleTimeout  = 120 * time.Second
	defaultGracefulShutdown   = 10 * time.Second
)

// Config captures runtime settings for the proxy.
type Config struct {
	ListenAddr              string
	Upstream                *url.URL
	APIKey                  string
	APISecret               string
	SessionHeader           string
	SessionValue            string
	RequestTimeout          time.Duration
	InsecureSkipVerify      bool
	LogLevel                string
	ServerReadTimeout       time.Duration
	ServerWriteTimeout      time.Duration
	ServerIdleTimeout       time.Duration
	GracefulShutdownTimeout time.Duration
}

// Load reads configuration from environment variables and validates required values.
func Load() (Config, error) {
	upstreamRaw := strings.TrimSpace(os.Getenv(envUpstreamURL))
	if upstreamRaw == "" {
		return Config{}, errors.New("MCP_UPSTREAM_URL is required")
	}

	upstream, err := url.Parse(upstreamRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MCP_UPSTREAM_URL: %w", err)
	}
	if !upstream.IsAbs() {
		return Config{}, errors.New("MCP_UPSTREAM_URL must be absolute (scheme://host)")
	}

	apiKey := strings.TrimSpace(os.Getenv(envAPIKey))
	if apiKey == "" {
		return Config{}, errors.New("MCP_API_KEY is required")
	}

	apiSecret := strings.TrimSpace(os.Getenv(envAPISecret))
	if apiSecret == "" {
		return Config{}, errors.New("MCP_API_SECRET is required")
	}

	cfg := Config{
		ListenAddr:              getString(envListenAddr, defaultListenAddr),
		Upstream:                upstream,
		APIKey:                  apiKey,
		APISecret:               apiSecret,
		SessionHeader:           getString(envSessionHeader, defaultSessionHeader),
		SessionValue:            strings.TrimSpace(os.Getenv(envSessionValue)),
		RequestTimeout:          getDuration(envRequestTimeout, defaultRequestTimeout),
		InsecureSkipVerify:      getBool(envInsecureSkipVerify, false),
		LogLevel:                strings.ToLower(getString(envLogLevel, defaultLogLevel)),
		ServerReadTimeout:       getDuration(envServerReadTimeout, defaultServerReadTimeout),
		ServerWriteTimeout:      getDuration(envServerWriteTimeout, defaultServerWriteTimeout),
		ServerIdleTimeout:       getDuration(envServerIdleTimeout, defaultServerIdleTimeout),
		GracefulShutdownTimeout: getDuration(envGracefulShutdown, defaultGracefulShutdown),
	}

	return cfg, nil
}

func getString(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return parsed
}

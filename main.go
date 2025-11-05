// Copyright © 2025 Prabhjot Singh Sethi, All Rights reserved
// Author: Prabhjot Singh Sethi <prabhjot.sethi@gmail.com>

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/go-core-stack/mcp-auth-proxy/pkg/config"
	"github.com/go-core-stack/mcp-auth-proxy/pkg/proxy"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Str("log_level", cfg.LogLevel).Msg("invalid log level")
	}
	log.Logger = log.Level(level)

	proxyHandler, err := proxy.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to construct proxy")
	}

	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      proxyHandler,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerIdleTimeout,
	}

	go func() {
		log.Info().
			Str("listen_addr", cfg.ListenAddr).
			Str("upstream", cfg.Upstream.String()).
			Msg("starting MCP auth proxy")
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("proxy server exited unexpectedly")
		}
	}()

	waitForShutdown(context.Background(), server, cfg.GracefulShutdownTimeout)
}

func waitForShutdown(ctx context.Context, srv *http.Server, timeout time.Duration) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop

	log.Info().Msg("shutting down MCP auth proxy")

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("graceful shutdown failed; forcing close")
		if closeErr := srv.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("forced close failed")
		}
	}

	log.Info().Msg("proxy stopped")
}

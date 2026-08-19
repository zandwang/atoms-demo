package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zand/atoms-demo/internal/app"
	"github.com/zand/atoms-demo/internal/config"
	"github.com/zand/atoms-demo/internal/store/sqlite"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load(".env")
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	modelStatus := cfg.ModelStatus()
	logger.Info(
		"starting atoms demo",
		"address", cfg.Address,
		"byok_available", modelStatus.Available,
		"model_timeout", cfg.GenerationTimeout(),
		"private_model_endpoints_allowed", cfg.AllowPrivateModelEndpoint,
	)

	repository, err := sqlite.Open(cfg.DataDir)
	if err != nil {
		logger.Error("open local store", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := repository.Close(); err != nil {
			logger.Error("close local store", "error", err)
		}
	}()

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           app.NewHandler(cfg, logger, repository),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-signalContext.Done():
		logger.Info("shutting down atoms demo")
		shutdownContext, cancel := context.WithTimeoutCause(
			context.Background(),
			10*time.Second,
			errors.New("graceful shutdown timed out"),
		)
		defer cancel()

		if err := server.Shutdown(shutdownContext); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}

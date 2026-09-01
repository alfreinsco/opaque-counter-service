package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"opaque-counter-service/internal/httpx"
	"opaque-counter-service/internal/redisx"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := loadConfig()

	redis := redisx.New(redisx.Config{
		Address:     cfg.redisAddress,
		Password:    cfg.redisPassword,
		DB:          cfg.redisDB,
		DialTimeout: 2 * time.Second,
		IOTimeout:   2 * time.Second,
		KeyPrefix:   cfg.keyPrefix,
	})
	defer redis.Close()

	handler := httpx.New(httpx.Config{
		PathPrefix: cfg.pathPrefix,
		MinToken:   cfg.minTokenLength,
		MaxToken:   cfg.maxTokenLength,
	}, redis, logger)

	server := &http.Server{
		Addr:              cfg.listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server_started", "listen", cfg.listenAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown_signal", "signal", sig.String())
	case err := <-errCh:
		logger.Error("server_failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown_failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server_stopped")
}

type config struct {
	listenAddress  string
	redisAddress   string
	redisPassword  string
	redisDB        int
	keyPrefix      string
	pathPrefix     string
	minTokenLength int
	maxTokenLength int
}

func loadConfig() config {
	return config{
		listenAddress:  env("LISTEN_ADDRESS", ":8080"),
		redisAddress:   env("REDIS_ADDRESS", "redis:6379"),
		redisPassword:  os.Getenv("REDIS_PASSWORD"),
		redisDB:        envInt("REDIS_DB", 0),
		keyPrefix:      env("KEY_PREFIX", "c:v1:"),
		pathPrefix:     env("PATH_PREFIX", "/x/"),
		minTokenLength: envInt("MIN_TOKEN_LENGTH", 20),
		maxTokenLength: envInt("MAX_TOKEN_LENGTH", 96),
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

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

	"github.com/Noussour/aup/apps/api/internal/config"
	apihttp "github.com/Noussour/aup/apps/api/internal/http"
	"github.com/Noussour/aup/apps/api/internal/repo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := repo.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := repo.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("redis connect", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	e := apihttp.NewServer(apihttp.Deps{
		Logger:   logger,
		PgPool:   pool,
		Redis:    rdb,
		AgentURL: cfg.AgentURL,
	})

	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server", "err", err)
			cancel()
		}
	}()
	logger.Info("api listening", "port", cfg.Port)

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}

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

	"mini-bank/internal/api"
	"mini-bank/internal/service"
	pg "mini-bank/internal/storage/postgres"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

// config holds the application configuration.
type config struct {
	Port       string
	DB_DSN     string
	JWT_KEY    string
	REDIS_ADDR string
	LogTokens  bool // If true, logs full reset tokens (development only)
}

func main() {
	// Setup structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		logger.Error("failed to load env", "err", err)
		os.Exit(1)
	}

	// Load configuration
	cfg := config{
		Port:       ":8080", // Default port
		DB_DSN:     os.Getenv("DATABASE_URL"),
		JWT_KEY:    os.Getenv("JWT_SECRET"),
		REDIS_ADDR: os.Getenv("REDIS_ADDR"),
		LogTokens:  os.Getenv("LOG_TOKENS") == "true",
	}
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		cfg.Port = ":" + portEnv
	}

	if cfg.DB_DSN == "" {
		logger.Error("DATABASE_URL environment variable is not set")
		os.Exit(1)
	}
	if cfg.JWT_KEY == "" {
		logger.Error("JWT_SECRET environment variable is not set")
		os.Exit(1)
	}

	db, err := pg.NewDB(cfg.DB_DSN)
	if err != nil {
		logger.Error("failed to connect to db", "err", err)
		os.Exit(1)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.REDIS_ADDR,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to redis", "err", err)
		os.Exit(1)
	}

	repo := pg.NewRepo(db)
	emailSender := service.NewLogEmailSender(logger, cfg.LogTokens)
	svc := service.New(repo, emailSender)
	a := api.NewAPI(svc, logger, rdb, cfg.JWT_KEY)
	handler := a.Router()
	handler = a.TimeoutMiddleware(handler, 15*time.Second)
	handler = a.LoggingMiddleware(handler)

	// graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Background worker for cleaning up expired tokens
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-quit:
				return
			case <-ticker.C:
				count, err := repo.CleanupExpiredPasswordResetTokens(context.Background())
				if err != nil {
					logger.Error("failed to cleanup expired tokens", "err", err)
				} else if count > 0 {
					logger.Info("cleaned up expired password reset tokens", "count", count)
				}
			}
		}
	}()

	// http server
	srv := &http.Server{
		Addr:         cfg.Port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// run server in goroutine
	go func() {
		logger.Info("listening on", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server listen failed", "err", err)
			os.Exit(1) // Exit if the server fails to start
		}
	}()

	<-quit
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "err", err)
		os.Exit(1)
	}

	// Close the database connection.
	if err := db.Close(); err != nil {
		logger.Error("database shutdown failed", "err", err)
	}

	logger.Info("server stopped gracefully")
}

package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/config"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httphandler"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/route"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/ormrepository"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/postgres"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/rabitmq"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

const (
	RefreshTokenTTL = time.Hour * 24 * 30
	AccessTokenTTL  = time.Minute * 30 // не очень определился, где это нужно хранить
	shutdownTimeout = 5 * time.Second
)

func Run() error {
	log := logging.Init()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx = logging.With(ctx, log)

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var pool *pgxpool.Pool
	pool, err = initPostgres(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	var sender *rabitmq.Publisher
	sender, err = initRabbitMQ(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sender.Close(); closeErr != nil {
			log.Error("close RabbitMQ publisher", slog.String("error", closeErr.Error()))
		}
	}()

	router := buildRouter(pool, sender, cfg)

	return runHTTPServer(ctx, log, router)
}

func initPostgres(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	return pool, nil
}

func initRabbitMQ(cfg config.Config) (*rabitmq.Publisher, error) {
	sender, err := rabitmq.NewPublisher(cfg.RabbitMQBaseURL)
	if err != nil {
		return nil, fmt.Errorf("create rabbitmq publisher: %w", err)
	}
	return sender, nil
}

func buildRouter(pool *pgxpool.Pool, sender *rabitmq.Publisher, cfg config.Config) http.Handler {
	repo := ormrepository.NewORMRepository(pool)
	jwtManager := access.NewManager(cfg.AccessTokenSecret, AccessTokenTTL)

	authService := services.NewAuthService(repo, repo, repo, sender, jwtManager, RefreshTokenTTL)
	sessionService := services.NewSessionService(repo)
	userService := services.NewUserService(repo, jwtManager)

	authHandler := httphandler.NewAuthHandler(authService)
	sessionHandler := httphandler.NewSessionHandler(sessionService)
	userHandler := httphandler.NewUserHandler(userService)

	return route.RegisterRoutes(authHandler, sessionHandler, userHandler, jwtManager)
}

func runHTTPServer(ctx context.Context, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		log.Info("HTTP server starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server failed",
				slog.String("error", err.Error()),
			)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down HTTP server",
		slog.String("addr", srv.Addr),
	)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("failed to shutdown HTTP server",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("shutdown http server: %w", err)
	}

	log.Info("HTTP server stopped")
	return nil
}

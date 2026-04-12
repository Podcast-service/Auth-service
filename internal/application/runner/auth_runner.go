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
		return err
	}

	var pool *pgxpool.Pool
	pool, err = postgres.NewPool(ctx, cfg)
	if err != nil {
		return err
	}

	defer pool.Close()

	repo := ormrepository.NewORMRepository(pool)

	var sender *rabitmq.Publisher
	sender, err = rabitmq.NewPublisher(cfg.RabbitMQBaseURL)
	if err != nil {

		fmt.Println("Failed to connect to RabbitMQ:", err, "URL:", cfg.RabbitMQBaseURL)
		return err
	}
	defer func() {
		closeErr := sender.Close()
		if closeErr != nil {
			log.Error("close RabbitMQ publisher",
				slog.String("error", closeErr.Error()),
			)
		}
	}()
	jwtManager := access.NewManager(cfg.AccessTokenSecret, AccessTokenTTL)

	authService := services.NewAuthService(repo, repo, repo, sender, jwtManager, RefreshTokenTTL)
	sessionService := services.NewSessionService(repo)
	userService := services.NewUserService(repo, jwtManager)

	authHandler := httphandler.NewAuthHandler(authService)
	sessionHandler := httphandler.NewSessionHandler(sessionService)
	userHandler := httphandler.NewUserHandler(userService)

	router := route.RegisterRoutes(authHandler, sessionHandler, userHandler, jwtManager)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		log.Info("HTTP server starting", "addr", httpServer.Addr)
		srvErr := httpServer.ListenAndServe()
		if srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			log.Error(
				"HTTP server failed",
				slog.String("error", srvErr.Error()),
			)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down HTTP server",
		slog.String("addr", httpServer.Addr),
	)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err = httpServer.Shutdown(shutdownCtx)
	if err != nil {
		log.Error(
			"failed to shutdown HTTP server",
			slog.String("error", err.Error()),
			slog.String("addr", httpServer.Addr),
		)
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}
	log.Info("HTTP server stopped")
	return nil
}

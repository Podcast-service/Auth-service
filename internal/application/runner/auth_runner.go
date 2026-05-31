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
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/config"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httphandler"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/route"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/kafkapkg"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/ormrepository"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/postgres"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/rabitmq"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/telemetry"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

const (
	RefreshTokenTTL      = time.Hour * 24 * 30
	AccessTokenTTL       = time.Minute * 30 // не очень определился, где это нужно хранить
	shutdownTimeout      = 5 * time.Second
	cleanupExpiredTokens = 24 * time.Hour
)

func Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lp, err := telemetry.InitLogger(ctx)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	log := logging.Init()
	ctx = logging.With(ctx, log)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if shutdownErr := lp.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("shutdown logger provider",
				slog.String("error", shutdownErr.Error()),
			)
		}
	}()

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var tp *sdktrace.TracerProvider
	tp, err = telemetry.InitTracer(ctx)
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if shutdownErr := tp.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("shutdown tracer provider",
				slog.String("error", shutdownErr.Error()),
			)
		}
	}()

	var mp *sdkmetric.MeterProvider
	mp, err = telemetry.InitMeter(ctx)
	if err != nil {
		return fmt.Errorf("init meter: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if shutdownErr := mp.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Error("shutdown meter provider",
				slog.String("error", shutdownErr.Error()),
			)
		}
	}()

	var pool *pgxpool.Pool
	pool, err = initPostgres(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	var rabbitPublisher *rabitmq.Publisher
	rabbitPublisher, err = initRabbitMQ(cfg)
	if err != nil {
		return err
	}
	defer func() {
		rbmErr := rabbitPublisher.Close()
		if rbmErr != nil {
			log.Error("close RabbitMQ publisher",
				slog.String("error", rbmErr.Error()),
			)
		}
	}()

	kafkaProducer := initKafka(cfg)
	defer func() {
		kfkErr := kafkaProducer.Close()
		if kfkErr != nil {
			log.Error("close kafka producer",
				slog.String("error", kfkErr.Error()),
			)
		}
	}()

	repo := ormrepository.NewORMRepository(pool)

	startTokenCleanup(ctx, log, repo)

	router := buildRouter(repo, rabbitPublisher, kafkaProducer, cfg)

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

func initKafka(cfg config.Config) *kafkapkg.Producer {
	kafkaProducer := kafkapkg.NewProducer(cfg.KafkaBrokers)
	return kafkaProducer
}

func buildRouter(
	repo *ormrepository.ORMRepository,
	rabbitPublisher *rabitmq.Publisher,
	kafkaProducer *kafkapkg.Producer,
	cfg config.Config,
) http.Handler {
	jwtManager := access.NewManager(cfg.AccessTokenSecret, AccessTokenTTL)

	authService := services.NewAuthService(repo, repo, repo, rabbitPublisher, kafkaProducer, jwtManager, RefreshTokenTTL)
	sessionService := services.NewSessionService(repo)
	userService := services.NewUserService(repo, jwtManager)

	authHandler := httphandler.NewAuthHandler(authService)
	sessionHandler := httphandler.NewSessionHandler(sessionService)
	userHandler := httphandler.NewUserHandler(userService)
	internalUserHandler := httphandler.NewInternalUserHandler(userService)

	return route.RegisterRoutes(authHandler, sessionHandler, userHandler, internalUserHandler, jwtManager)
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

func startTokenCleanup(ctx context.Context, log *slog.Logger, repo *ormrepository.ORMRepository) {
	go func() {
		ticker := time.NewTicker(cleanupExpiredTokens)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := repo.DeleteExpiredTokens(context.Background()); err != nil {
					log.Error("failed to delete expired tokens",
						slog.String("error", err.Error()),
					)
				} else {
					log.Info("expired tokens cleanup completed")
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

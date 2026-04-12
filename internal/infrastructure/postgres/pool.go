package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/config"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/migration"
)

const (
	contextTimeout = 5 * time.Second
)

func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	log := logging.FromContext(ctx)
	log.Info("Connecting to PostgreSQL database...")

	sourceMigrationURL := "file:///app/migrations"
	//sourceMigrationURL := "file://migrations"
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password, cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	err := migration.RunMigration(dsn, sourceMigrationURL)
	if err != nil {
		err = fmt.Errorf("run migration: %w", err)
		log.Error("Failed to run database migrations",
			slog.String("error", err.Error()),
		)
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, contextTimeout)

	defer cancel()

	var poolConfig *pgxpool.Config
	poolConfig, err = pgxpool.ParseConfig(dsn)
	if err != nil {
		err = fmt.Errorf("parse database config: %w", err)
		log.Error("Failed to parse database configuration",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	var pool *pgxpool.Pool
	pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		err = fmt.Errorf("make database pool: %w", err)
		log.Error("Failed to create database connection pool",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		err = fmt.Errorf("ping database: %w", err)
		log.Error("Failed to ping database",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	log.Info("Successfully connected to database")
	return pool, nil
}

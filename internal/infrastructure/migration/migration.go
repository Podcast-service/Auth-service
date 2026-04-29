package migration

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	// Регистрируем драйвер PostgreSQL для migrate
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// Регистрируем источник файлов для migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigration(dataBaseURL string, sourceURL string) error {
	slog.Info("starting migration",
		slog.String("sourceURL", sourceURL),
		slog.String("dataBaseURL", dataBaseURL),
	)
	m, err := migrate.New(sourceURL, dataBaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to run migration: %w", err)
	}
	return nil
}

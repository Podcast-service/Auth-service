package config

import (
	"errors"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AccessTokenSecret string
	RabbitMQBaseURL   string
	Database          DatabaseConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func LoadConfig() (Config, error) {
	err := godotenv.Load()
	if err != nil {
		slog.Warn(
			"failed to parse config or .env file not found",
			slog.String("error", err.Error()),
		)
	}

	accessTokenSecret := os.Getenv("ACCESS_TOKEN_SECRET")
	if accessTokenSecret == "" {
		err = errors.New("ACCESS_TOKEN_SECRET environment variable is required")
		slog.Error("ACCESS_TOKEN_SECRET environment variable not set",
			slog.String("error", err.Error()),
		)
		return Config{}, err
	}

	rabbitMQBaseURL := os.Getenv("RABBITMQ_BASE_URL")
	if rabbitMQBaseURL == "" {
		err = errors.New("RABBITMQ_BASE_URL environment variable is required")
		slog.Error("RABBITMQ_BASE_URL environment variable not set",
			slog.String("error", err.Error()),
		)
		return Config{}, err
	}

	dbConfig := DatabaseConfig{}
	dbConfig, err = LoadDBConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		AccessTokenSecret: accessTokenSecret,
		RabbitMQBaseURL:   rabbitMQBaseURL,
		Database:          dbConfig,
	}, nil
}

func LoadDBConfig() (DatabaseConfig, error) {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		err := errors.New("DB_HOST is required")
		slog.Error("DB_HOST environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		err := errors.New("DB_PORT is required")
		slog.Error("DB_PORT environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbUser := os.Getenv("POSTGRES_USER")
	if dbUser == "" {
		err := errors.New("POSTGRES_PASSWORD is required")
		slog.Error("POSTGRES_PASSWORD environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	if dbPassword == "" {
		err := errors.New("POSTGRES_PASSWORD is required")
		slog.Error("POSTGRES_PASSWORD environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		err := errors.New("POSTGRES_DB is required")
		slog.Error("POSTGRES_DB environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		slog.Warn("DB_SSLMODE environment variable not set, defaulting to 'disable'")
		dbSSLMode = "disable"
	}
	return DatabaseConfig{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		Name:     dbName,
		SSLMode:  dbSSLMode,
	}, nil
}

package config

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AccessTokenSecret      string
	RabbitMQBaseURL        string
	KafkaBrokers           []string
	KafkaTopicUserRegister string
	Database               DatabaseConfig
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

	kafkaBrokersRaw := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokersRaw == "" {
		return Config{}, errors.New("KAFKA_BROKERS environment variable is required")
	}
	kafkaBrokers := strings.Split(kafkaBrokersRaw, ",")

	var dbConfig DatabaseConfig
	dbConfig, err = LoadDBConfig()
	if err != nil {
		return Config{}, err
	}
	return Config{
		AccessTokenSecret: accessTokenSecret,
		RabbitMQBaseURL:   rabbitMQBaseURL,
		KafkaBrokers:      kafkaBrokers,
		Database:          dbConfig,
	}, nil
}

func LoadDBConfig() (DatabaseConfig, error) {
	dbHost := os.Getenv("AUTH_DB_HOST")
	if dbHost == "" {
		err := errors.New("AUTH_DB_HOST is required")
		slog.Error("DB_HOST environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbPort := os.Getenv("AUTH_DB_PORT")
	if dbPort == "" {
		err := errors.New("AUTH_DB_PORT is required")
		slog.Error("DB_PORT environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbUser := os.Getenv("AUTH_DB_USER")
	if dbUser == "" {
		err := errors.New("AUTH_DB_USER is required")
		slog.Error("AUTH_DB_USER environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbPassword := os.Getenv("AUTH_DB_PASSWORD")
	if dbPassword == "" {
		err := errors.New("AUTH_DB_PASSWORD is required")
		slog.Error("AUTH_DB_PASSWORD environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbName := os.Getenv("AUTH_DB_NAME")
	if dbName == "" {
		err := errors.New("AUTH_DB_NAME is required")
		slog.Error("AUTH_DB_NAME environment variable not set",
			slog.String("error", err.Error()),
		)
		return DatabaseConfig{}, err
	}
	dbSSLMode := os.Getenv("AUTH_DB_SSLMODE")
	if dbSSLMode == "" {
		slog.Warn("AUTH_DB_SSLMODE environment variable not set, defaulting to 'disable'")
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

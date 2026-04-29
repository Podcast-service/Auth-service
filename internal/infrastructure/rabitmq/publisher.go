package rabitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

const (
	QueueName = "email.queue"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewPublisher(ampqURL string) (*Publisher, error) {
	conn, err := amqp.Dial(ampqURL)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	var channel *amqp.Channel
	channel, err = conn.Channel()
	if err != nil {
		conErr := conn.Close()
		if conErr != nil {
			slog.Error("close RabbitMQ connection",
				slog.String("error", conErr.Error()),
			)
		}
		return nil, fmt.Errorf("create RabbitMQ channel: %w", err)
	}

	_, err = channel.QueueDeclare(
		QueueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		chErr := channel.Close()
		if chErr != nil {
			slog.Error("close RabbitMQ channel",
				slog.String("error", chErr.Error()),
			)
		}
		conErr := conn.Close()
		if conErr != nil {
			slog.Error("close RabbitMQ connection",
				slog.String("error", conErr.Error()),
			)
		}
		return nil, fmt.Errorf("declare RabbitMQ queue: %w", err)
	}

	return &Publisher{
		conn:    conn,
		channel: channel,
	}, nil

}

func (p *Publisher) SendMessage(ctx context.Context, message any) error {
	log := logging.FromContext(ctx)
	body, err := json.Marshal(message)
	if err != nil {
		log.Error("Failed to marshal message",
			slog.Any("message", message),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("marshal message: %w", err)
	}
	err = p.channel.PublishWithContext(ctx,
		"", // exchange - пустая строка означает default exchange
		QueueName,
		false, // mandatory - не возвращать сообщение, если подходящей очереди нет
		false, // immediate - не требовать немдленного консьюмера
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		log.Error("Failed to publish message",
			slog.String("body", string(body)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("publish message: %w", err)
	}
	slog.Info("message published to rabbitmq",
		slog.String("queue", QueueName),
		slog.String("body", string(body)),
	)
	return nil
}

func (p *Publisher) Close() error {
	err := p.channel.Close()
	if err != nil {
		slog.Error("close RabbitMQ channel",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("close RabbitMQ channel: %w", err)
	}
	err = p.conn.Close()
	if err != nil {
		slog.Error("close RabbitMQ connection",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("close RabbitMQ connection: %w", err)
	}
	return nil
}

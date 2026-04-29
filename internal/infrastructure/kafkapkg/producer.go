package kafkapkg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

const (
	kafkaTimeout = time.Second * 10
	kafkaTopic   = "podcast.user.register"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        kafkaTopic,
			Balancer:     &kafka.LeastBytes{},
			WriteTimeout: kafkaTimeout,
			ReadTimeout:  kafkaTimeout,
		},
	}
}

func (p *Producer) SendMessage(ctx context.Context, message any) error {
	log := logging.FromContext(ctx)
	data, err := json.Marshal(message)
	if err != nil {
		log.Error("Failed to marshal register user message")
		return fmt.Errorf("marshal register user message: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Value: data,
	})
	if err != nil {
		log.Error("Failed to send register user message")
		return fmt.Errorf("send register user message: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	err := p.writer.Close()
	if err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}

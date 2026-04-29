package services

import (
	"context"
)

type Sender interface {
	SendMessage(ctx context.Context, message any) error
	Close() error
}

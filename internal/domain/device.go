package domain

import (
	"time"

	"github.com/google/uuid"
)

type Device struct {
	RefreshTokenID uuid.UUID
	DeviceName     string
	IPAddress      string
	UserAgent      string
	CreatedAt      time.Time
	LastUsedAt     time.Time
}

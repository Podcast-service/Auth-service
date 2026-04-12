package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  string
	DeviceName string
	IPAddress  string
	UserAgent  string

	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	Revoked    bool
}

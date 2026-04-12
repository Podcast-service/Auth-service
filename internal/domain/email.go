package domain

import (
	"time"

	"github.com/google/uuid"
)

type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Code      string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

type EmailVerifyToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Code      string
	ExpiresAt time.Time
	Used      bool
	CreatedAt time.Time
}

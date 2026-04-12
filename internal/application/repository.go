package application

import (
	"context"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

type AuthRepository interface {
	RegisterUser(ctx context.Context, email, passwordHash string, verifyToken domain.EmailVerifyToken) (uuid.UUID, error)
	ConfirmEmail(ctx context.Context, token domain.EmailVerifyToken) error
	ResetPassword(ctx context.Context, token domain.PasswordResetToken, newPasswordHash string) error

	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error)

	GetEmailVerifyToken(ctx context.Context, email string, code string) (domain.EmailVerifyToken, error)
	CreateEmailVerifyToken(ctx context.Context, token domain.EmailVerifyToken) error

	CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error
	GetPasswordResetToken(ctx context.Context, email string, code string) (domain.PasswordResetToken, error)
}

type SessionRepository interface {
	CreateRefreshToken(ctx context.Context, token domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
	GetUserDevices(ctx context.Context, userID uuid.UUID) ([]domain.Device, error)
	UpdateLastUsed(ctx context.Context, tokenID uuid.UUID) error
}

type UserRepository interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error)
	AssignRole(ctx context.Context, userID uuid.UUID, roleName string) error
}

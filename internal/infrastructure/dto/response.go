package dto

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type Device struct {
	DeviceName     string    `json:"device_name"`
	IPAddress      string    `json:"ip_address"`
	UserAgent      string    `json:"user_agent"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     time.Time `json:"last_used_at"`
	RefreshTokenID uuid.UUID `json:"refresh_token_id"`
}

type RolesResponse struct {
	Roles []string `json:"roles"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type EmailNotVerifiedResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type AdminUserResponse struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	Roles         []string  `json:"roles"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AdminPageResponse[T any] struct {
	Items         []T   `json:"items"`
	Page          int   `json:"page"`
	Size          int   `json:"size"`
	TotalElements int64 `json:"total_elements"`
	TotalPages    int   `json:"total_pages"`
}

type AdminRoleMutationResponse struct {
	UserID  uuid.UUID `json:"user_id"`
	Roles   []string  `json:"roles"`
	Changed bool      `json:"changed"`
}

package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrAlreadyCodeUsed    = errors.New("token already used")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrForbidden          = errors.New("forbidden")
)

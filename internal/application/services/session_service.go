package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/application"
	"github.com/Podcast-service/Auth-service/internal/application/generator"
	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

type SessionService interface {
	Logout(ctx context.Context, userID uuid.UUID, req dto.LogoutRequest) error
	LogoutAll(ctx context.Context, userID uuid.UUID) error
	Devices(ctx context.Context, userID uuid.UUID) ([]dto.Device, error)
}

type sessionService struct {
	repo application.SessionRepository
}

func NewSessionService(repo application.SessionRepository) SessionService {
	return &sessionService{repo: repo}
}

func (s sessionService) Logout(ctx context.Context, userID uuid.UUID, req dto.LogoutRequest) error {
	log := logging.FromContext(ctx)

	tokenHash := generator.HashToken(req.RefreshToken)
	storedToken, err := s.repo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("refresh token not found",
				slog.String("token_hash", tokenHash),
			)
			return domain.ErrInvalidCredentials
		}
		return fmt.Errorf("get refresh token by id: %w", err)
	}

	if storedToken.UserID != userID {
		log.Warn("refresh token does not belong to user",
			slog.String("refresh_token", tokenHash),
			slog.String("user_id", userID.String()),
		)
		return domain.ErrForbidden
	}

	err = s.repo.RevokeRefreshToken(ctx, storedToken.ID)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	log.Info("user logged out",
		slog.String("user_id", userID.String()),
		slog.String("refresh_token_id", storedToken.ID.String()),
	)

	return nil
}

func (s sessionService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	log := logging.FromContext(ctx)
	err := s.repo.RevokeAllUserTokens(ctx, userID)
	if err != nil {
		return fmt.Errorf("revoke all refresh tokens: %w", err)
	}

	log.Info("user logged out from all devices",
		slog.String("user_id", userID.String()),
	)

	return nil
}

func (s sessionService) Devices(ctx context.Context, userID uuid.UUID) ([]dto.Device, error) {
	log := logging.FromContext(ctx)

	devices, err := s.repo.GetUserDevices(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user devices: %w", err)
	}
	var resp []dto.Device
	for _, d := range devices {
		resp = append(resp, dto.Device{
			DeviceName:     d.DeviceName,
			IPAddress:      d.IPAddress,
			UserAgent:      d.UserAgent,
			CreatedAt:      d.CreatedAt,
			LastUsedAt:     d.LastUsedAt,
			RefreshTokenID: d.RefreshTokenID,
		})
	}

	log.Info("devices received",
		slog.String("user_id", userID.String()),
		slog.Int("device_count", len(resp)),
	)

	return resp, nil
}

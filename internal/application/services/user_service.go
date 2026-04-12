package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/application"
	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

type UserService interface {
	Roles(ctx context.Context, userID uuid.UUID) (dto.RolesResponse, error)
	UpdateRoles(ctx context.Context, userID uuid.UUID, req dto.UpdateRolesRequest) (dto.AccessTokenResponse, error)
}

type userService struct {
	repo       application.UserRepository
	jwtManager *access.Manager
}

func NewUserService(repo application.UserRepository, jwtManager *access.Manager) UserService {
	return &userService{repo: repo, jwtManager: jwtManager}
}

func (u userService) Roles(ctx context.Context, userID uuid.UUID) (dto.RolesResponse, error) {
	log := logging.FromContext(ctx)
	roles, err := u.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return dto.RolesResponse{}, fmt.Errorf("get user roles: %w", err)
	}

	log.Info("roles received",
		slog.String("user_id", userID.String()),
		slog.Int("role_count", len(roles)),
	)

	return dto.RolesResponse{Roles: roles}, nil
}

func (u userService) UpdateRoles(ctx context.Context, userID uuid.UUID, req dto.UpdateRolesRequest) (dto.AccessTokenResponse, error) {
	log := logging.FromContext(ctx)
	err := u.repo.AssignRole(ctx, userID, req.RoleName)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			log.Warn("user already has role",
				slog.String("user_id", userID.String()),
				slog.String("role_name", req.RoleName),
			)
			return dto.AccessTokenResponse{}, domain.ErrAlreadyExists
		}
		return dto.AccessTokenResponse{}, fmt.Errorf("assign role: %w", err)
	}

	var user domain.User
	user, err = u.repo.GetUserByID(ctx, userID)
	if err != nil {
		return dto.AccessTokenResponse{}, fmt.Errorf("get user by id: %w", err)
	}

	var roles []string
	roles, err = u.repo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return dto.AccessTokenResponse{}, fmt.Errorf("get user roles: %w", err)
	}

	var accessToken string
	accessToken, err = u.jwtManager.GenerateAccessToken(user.ID, user.Email, roles)
	if err != nil {
		return dto.AccessTokenResponse{}, fmt.Errorf("generate access token: %w", err)
	}

	log.Info("user role updated",
		slog.String("user_id", user.ID.String()),
		slog.String("new_role", req.RoleName),
	)

	return dto.AccessTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(u.jwtManager.AccessTokenTTL.Seconds()),
	}, nil
}

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
)


type AdminService interface {
	ListUsers(ctx context.Context, filter domain.AdminUserFilter) (dto.AdminPageResponse[dto.AdminUserResponse], error)
	GetUser(ctx context.Context, userID uuid.UUID) (dto.AdminUserResponse, error)
	AddRole(ctx context.Context, userID uuid.UUID, roleName string) (dto.AdminRoleMutationResponse, error)
	RemoveAdminRole(ctx context.Context, userID uuid.UUID) (dto.AdminRoleMutationResponse, error)
}

type adminService struct {
	repo application.UserRepository
}

func NewAdminService(repo application.UserRepository) AdminService {
	return &adminService{repo: repo}
}

func (a adminService) ListUsers(ctx context.Context, filter domain.AdminUserFilter) (dto.AdminPageResponse[dto.AdminUserResponse], error) {
	log := logging.FromContext(ctx)

	filter = normalizeFilter(filter)

	users, total, err := a.repo.ListUsers(ctx, filter)
	if err != nil {
		return dto.AdminPageResponse[dto.AdminUserResponse]{}, fmt.Errorf("list users: %w", err)
	}

	items := make([]dto.AdminUserResponse, 0, len(users))
	for _, u := range users {
		items = append(items, toAdminUserResponse(u))
	}

	totalPages := 0
	if filter.Size > 0 {
		totalPages = int((total + int64(filter.Size) - 1) / int64(filter.Size))
	}

	log.Info("admin listed users",
		slog.Int("returned", len(items)),
		slog.Int64("total_elements", total),
	)

	return dto.AdminPageResponse[dto.AdminUserResponse]{
		Items:         items,
		Page:          filter.Page,
		Size:          filter.Size,
		TotalElements: total,
		TotalPages:    totalPages,
	}, nil
}

func (a adminService) GetUser(ctx context.Context, userID uuid.UUID) (dto.AdminUserResponse, error) {
	log := logging.FromContext(ctx)

	user, err := a.repo.GetUserWithRolesByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return dto.AdminUserResponse{}, domain.ErrNotFound
		}
		return dto.AdminUserResponse{}, fmt.Errorf("get user with roles by id: %w", err)
	}

	log.Info("admin fetched user", slog.String("user_id", userID.String()))

	return toAdminUserResponse(user), nil
}

func (a adminService) AddRole(ctx context.Context, userID uuid.UUID, roleName string) (dto.AdminRoleMutationResponse, error) {
	log := logging.FromContext(ctx)

	if !domain.IsAllowedRole(roleName) {
		log.Warn("admin requested invalid role assignment",
			slog.String("user_id", userID.String()),
			slog.String("role_name", roleName),
		)
		return dto.AdminRoleMutationResponse{}, domain.ErrInvalidRole
	}

	if _, err := a.repo.GetUserWithRolesByID(ctx, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return dto.AdminRoleMutationResponse{}, domain.ErrNotFound
		}
		return dto.AdminRoleMutationResponse{}, fmt.Errorf("get user with roles by id: %w", err)
	}

	changed := true
	err := a.repo.AssignRole(ctx, userID, roleName)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			changed = false
		} else {
			return dto.AdminRoleMutationResponse{}, fmt.Errorf("assign role: %w", err)
		}
	}

	resp, err := a.freshRoleMutationResponse(ctx, userID, changed)
	if err != nil {
		return dto.AdminRoleMutationResponse{}, err
	}

	log.Info("admin added role",
		slog.String("user_id", userID.String()),
		slog.String("role_name", roleName),
		slog.Bool("changed", changed),
	)

	return resp, nil
}

func (a adminService) RemoveAdminRole(ctx context.Context, userID uuid.UUID) (dto.AdminRoleMutationResponse, error) {
	log := logging.FromContext(ctx)

	user, err := a.repo.GetUserWithRolesByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return dto.AdminRoleMutationResponse{}, domain.ErrNotFound
		}
		return dto.AdminRoleMutationResponse{}, fmt.Errorf("get user with roles by id: %w", err)
	}

	if !containsRole(user.Roles, domain.RoleAdmin) {
		return a.freshRoleMutationResponse(ctx, userID, false)
	}

	adminCount, err := a.repo.CountUsersByRole(ctx, domain.RoleAdmin)
	if err != nil {
		return dto.AdminRoleMutationResponse{}, fmt.Errorf("count users by role: %w", err)
	}
	if adminCount <= 1 {
		log.Warn("refused to remove the last admin", slog.String("user_id", userID.String()))
		return dto.AdminRoleMutationResponse{}, domain.ErrLastAdmin
	}

	changed, err := a.repo.RemoveRole(ctx, userID, domain.RoleAdmin)
	if err != nil {
		return dto.AdminRoleMutationResponse{}, fmt.Errorf("remove role: %w", err)
	}

	resp, err := a.freshRoleMutationResponse(ctx, userID, changed)
	if err != nil {
		return dto.AdminRoleMutationResponse{}, err
	}

	log.Info("admin removed admin role",
		slog.String("user_id", userID.String()),
		slog.Bool("changed", changed),
	)

	return resp, nil
}

func (a adminService) freshRoleMutationResponse(ctx context.Context, userID uuid.UUID, changed bool) (dto.AdminRoleMutationResponse, error) {
	roles, err := a.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return dto.AdminRoleMutationResponse{}, fmt.Errorf("get user roles: %w", err)
	}
	return dto.AdminRoleMutationResponse{
		UserID:  userID,
		Roles:   normalizeRoles(roles),
		Changed: changed,
	}, nil
}

func normalizeFilter(filter domain.AdminUserFilter) domain.AdminUserFilter {
	if filter.Page < 0 {
		filter.Page = domain.DefaultPageNo
	}
	if filter.Size <= 0 {
		filter.Size = domain.DefaultSize
	}
	if filter.Size > domain.MaxPageSize {
		filter.Size = domain.MaxPageSize
	}
	if !domain.IsAllowedSort(filter.Sort) {
		filter.Sort = domain.DefaultSort
	}
	return filter
}

func toAdminUserResponse(u domain.UserWithRoles) dto.AdminUserResponse {
	return dto.AdminUserResponse{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerified,
		Roles:         normalizeRoles(u.Roles),
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func normalizeRoles(roles []string) []string {
	if roles == nil {
		return []string{}
	}
	return roles
}

func containsRole(roles []string, target string) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}

package route_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httphandler"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/route"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

const testSecret = "test-secret-key"

type stubAdminService struct{}

func (stubAdminService) ListUsers(_ context.Context, _ domain.AdminUserFilter) (dto.AdminPageResponse[dto.AdminUserResponse], error) {
	return dto.AdminPageResponse[dto.AdminUserResponse]{Items: []dto.AdminUserResponse{}}, nil
}

func (stubAdminService) GetUser(_ context.Context, _ uuid.UUID) (dto.AdminUserResponse, error) {
	return dto.AdminUserResponse{}, nil
}

func (stubAdminService) AddRole(_ context.Context, _ uuid.UUID, _ string) (dto.AdminRoleMutationResponse, error) {
	return dto.AdminRoleMutationResponse{}, nil
}

func (stubAdminService) RemoveAdminRole(_ context.Context, _ uuid.UUID) (dto.AdminRoleMutationResponse, error) {
	return dto.AdminRoleMutationResponse{}, nil
}

func newTestRouter() http.Handler {
	jwtManager := access.NewManager(testSecret, time.Hour)
	adminHandler := httphandler.NewAdminHandler(stubAdminService{})

	return route.RegisterRoutes(
		httphandler.NewAuthHandler(nil),
		httphandler.NewSessionHandler(nil),
		httphandler.NewUserHandler(nil),
		httphandler.NewInternalUserHandler(nil),
		adminHandler,
		jwtManager,
	)
}

func tokenWithRoles(t *testing.T, roles []string) string {
	t.Helper()
	jwtManager := access.NewManager(testSecret, time.Hour)
	token, err := jwtManager.GenerateAccessToken(uuid.New(), "actor@example.com", roles)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

func TestAdminRoutes_RequireAuth(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAdminRoutes_NonAdminForbidden(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithRoles(t, []string{domain.RoleUser}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestAdminRoutes_AdminAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenWithRoles(t, []string{domain.RoleUser, domain.RoleAdmin}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", rec.Code)
	}
}

func TestAdminRoutes_InvalidTokenUnauthorized(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rec.Code)
	}
}

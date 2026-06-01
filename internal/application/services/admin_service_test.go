package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

type fakeUserRepo struct {
	users map[uuid.UUID]*fakeUser
}

type fakeUser struct {
	email         string
	emailVerified bool
	roles         []string
	createdAt     time.Time
	updatedAt     time.Time
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*fakeUser)}
}

func (f *fakeUserRepo) add(id uuid.UUID, email string, verified bool, roles ...string) {
	f.users[id] = &fakeUser{
		email:         email,
		emailVerified: verified,
		roles:         append([]string{}, roles...),
		createdAt:     time.Now(),
		updatedAt:     time.Now(),
	}
}

func (f *fakeUserRepo) GetUserByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return domain.User{ID: id, Email: u.email, EmailVerified: u.emailVerified}, nil
}

func (f *fakeUserRepo) GetUserRoles(_ context.Context, userID uuid.UUID) ([]string, error) {
	u, ok := f.users[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := append([]string{}, u.roles...)
	sort.Strings(out)
	return out, nil
}

func (f *fakeUserRepo) AssignRole(_ context.Context, userID uuid.UUID, roleName string) error {
	u, ok := f.users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	for _, r := range u.roles {
		if r == roleName {
			return domain.ErrAlreadyExists
		}
	}
	u.roles = append(u.roles, roleName)
	return nil
}

func (f *fakeUserRepo) RemoveRole(_ context.Context, userID uuid.UUID, roleName string) (bool, error) {
	u, ok := f.users[userID]
	if !ok {
		return false, domain.ErrNotFound
	}
	for i, r := range u.roles {
		if r == roleName {
			u.roles = append(u.roles[:i], u.roles[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUserRepo) CountUsersByRole(_ context.Context, roleName string) (int64, error) {
	var count int64
	for _, u := range f.users {
		for _, r := range u.roles {
			if r == roleName {
				count++
				break
			}
		}
	}
	return count, nil
}

func (f *fakeUserRepo) GetUserWithRolesByID(_ context.Context, userID uuid.UUID) (domain.UserWithRoles, error) {
	u, ok := f.users[userID]
	if !ok {
		return domain.UserWithRoles{}, domain.ErrNotFound
	}
	roles := append([]string{}, u.roles...)
	sort.Strings(roles)
	return domain.UserWithRoles{
		ID:            userID,
		Email:         u.email,
		EmailVerified: u.emailVerified,
		Roles:         roles,
		CreatedAt:     u.createdAt,
		UpdatedAt:     u.updatedAt,
	}, nil
}

func (f *fakeUserRepo) ListUsers(_ context.Context, filter domain.AdminUserFilter) ([]domain.UserWithRoles, int64, error) {
	var matched []domain.UserWithRoles
	for id, u := range f.users {
		if filter.Query != "" && !strings.Contains(u.email, filter.Query) {
			continue
		}
		if filter.EmailVerified != nil && u.emailVerified != *filter.EmailVerified {
			continue
		}
		if filter.Role != "" && !containsRole(u.roles, filter.Role) {
			continue
		}
		roles := append([]string{}, u.roles...)
		sort.Strings(roles)
		matched = append(matched, domain.UserWithRoles{
			ID: id, Email: u.email, EmailVerified: u.emailVerified, Roles: roles,
			CreatedAt: u.createdAt, UpdatedAt: u.updatedAt,
		})
	}

	total := int64(len(matched))

	start := filter.Page * filter.Size
	if start > len(matched) {
		start = len(matched)
	}
	end := start + filter.Size
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func TestAdminService_AddRole(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	userID := uuid.New()
	repo.add(userID, "user@example.com", true, domain.RoleUser)

	svc := NewAdminService(repo)

	resp, err := svc.AddRole(ctx, userID, domain.RoleAuthor)
	if err != nil {
		t.Fatalf("AddRole returned error: %v", err)
	}
	if !resp.Changed {
		t.Errorf("expected changed=true on first assignment")
	}
	if !containsRole(resp.Roles, domain.RoleAuthor) {
		t.Errorf("expected roles to include author, got %v", resp.Roles)
	}

	resp, err = svc.AddRole(ctx, userID, domain.RoleAuthor)
	if err != nil {
		t.Fatalf("idempotent AddRole returned error: %v", err)
	}
	if resp.Changed {
		t.Errorf("expected changed=false on repeated assignment")
	}
}

func TestAdminService_AddRole_InvalidRole(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	userID := uuid.New()
	repo.add(userID, "user@example.com", true, domain.RoleUser)

	svc := NewAdminService(repo)

	_, err := svc.AddRole(ctx, userID, "superuser")
	if !errors.Is(err, domain.ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestAdminService_AddRole_UnknownUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewAdminService(repo)

	_, err := svc.AddRole(ctx, uuid.New(), domain.RoleAuthor)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminService_GetUser_UnknownUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewAdminService(repo)

	_, err := svc.GetUser(ctx, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminService_RemoveAdminRole(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	admin1 := uuid.New()
	admin2 := uuid.New()
	repo.add(admin1, "admin1@example.com", true, domain.RoleUser, domain.RoleAdmin)
	repo.add(admin2, "admin2@example.com", true, domain.RoleUser, domain.RoleAdmin)

	svc := NewAdminService(repo)

	resp, err := svc.RemoveAdminRole(ctx, admin1)
	if err != nil {
		t.Fatalf("RemoveAdminRole returned error: %v", err)
	}
	if !resp.Changed {
		t.Errorf("expected changed=true when removing admin role")
	}
	if containsRole(resp.Roles, domain.RoleAdmin) {
		t.Errorf("expected admin role removed, got %v", resp.Roles)
	}
}

func TestAdminService_RemoveAdminRole_Idempotent(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	adminID := uuid.New()
	plainID := uuid.New()
	repo.add(adminID, "admin@example.com", true, domain.RoleAdmin)
	repo.add(plainID, "user@example.com", true, domain.RoleUser)

	svc := NewAdminService(repo)

	resp, err := svc.RemoveAdminRole(ctx, plainID)
	if err != nil {
		t.Fatalf("RemoveAdminRole returned error: %v", err)
	}
	if resp.Changed {
		t.Errorf("expected changed=false when user has no admin role")
	}
}

func TestAdminService_RemoveAdminRole_LastAdmin(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	adminID := uuid.New()
	repo.add(adminID, "admin@example.com", true, domain.RoleUser, domain.RoleAdmin)

	svc := NewAdminService(repo)

	_, err := svc.RemoveAdminRole(ctx, adminID)
	if !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}

	// The admin role must still be present.
	roles, _ := repo.GetUserRoles(ctx, adminID)
	if !containsRole(roles, domain.RoleAdmin) {
		t.Errorf("last admin must keep the admin role, got %v", roles)
	}
}

func TestAdminService_RemoveAdminRole_UnknownUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	svc := NewAdminService(repo)

	_, err := svc.RemoveAdminRole(ctx, uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminService_ListUsers(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	for i := 0; i < 3; i++ {
		repo.add(uuid.New(), "user@example.com", true, domain.RoleUser)
	}

	svc := NewAdminService(repo)

	page, err := svc.ListUsers(ctx, domain.AdminUserFilter{Page: 0, Size: 2})
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if page.TotalElements != 3 {
		t.Errorf("expected total_elements=3, got %d", page.TotalElements)
	}
	if page.TotalPages != 2 {
		t.Errorf("expected total_pages=2, got %d", page.TotalPages)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items on first page, got %d", len(page.Items))
	}
}

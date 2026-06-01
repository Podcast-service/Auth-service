package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleUser   = "user"
	RoleAuthor = "author"
	RoleAdmin  = "admin"
)

var AllowedRoles = map[string]struct{}{
	RoleUser:   {},
	RoleAuthor: {},
	RoleAdmin:  {},
}

func IsAllowedRole(role string) bool {
	_, ok := AllowedRoles[role]
	return ok
}

const (
	SortDateDesc  = "DATE_DESC"
	SortDateAsc   = "DATE_ASC"
	SortEmailAsc  = "EMAIL_ASC"
	DefaultSort   = SortDateDesc
	DefaultSize   = 20
	MaxPageSize   = 100
	DefaultPageNo = 0
)

func IsAllowedSort(sort string) bool {
	switch sort {
	case SortDateDesc, SortDateAsc, SortEmailAsc:
		return true
	default:
		return false
	}
}

type UserWithRoles struct {
	ID            uuid.UUID
	Email         string
	EmailVerified bool
	Roles         []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AdminUserFilter struct {
	Query         string
	Role          string
	EmailVerified *bool
	Page          int
	Size          int
	Sort          string
}

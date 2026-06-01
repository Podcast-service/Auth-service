package ormrepository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

func (r *ORMRepository) AssignRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	err := r.assignRole(ctx, r.pool, userID, roleName)
	if err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	return nil
}

func (r *ORMRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("r.name").
		From("roles r").
		Join("user_roles ur ON ur.role_id = r.id").
		Where(sq.Eq{"ur.user_id": userID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build GetUserRoles query: %w", err)
	}
	var rows pgx.Rows
	rows, err = r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("execute GetUserRoles query: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, fmt.Errorf("scan GetUserRoles row: %w", err)
		}
		roles = append(roles, name)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("get user roles: %w", rows.Err())
	}
	return roles, nil
}

const rolesAggColumn = "COALESCE(array_agg(r.name ORDER BY r.name) " +
	"FILTER (WHERE r.name IS NOT NULL), '{}') AS roles"

func (r *ORMRepository) ListUsers(ctx context.Context, filter domain.AdminUserFilter) ([]domain.UserWithRoles, int64, error) {
	conds := adminUserFilterConds(filter)

	listBuilder := psql.
		Select("u.id", "u.email", "u.email_verified", "u.created_at", "u.updated_at", rolesAggColumn).
		From("users u").
		LeftJoin("user_roles ur ON ur.user_id = u.id").
		LeftJoin("roles r ON r.id = ur.role_id").
		GroupBy("u.id")
	if len(conds) > 0 {
		listBuilder = listBuilder.Where(conds)
	}

	switch filter.Sort {
	case domain.SortDateAsc:
		listBuilder = listBuilder.OrderBy("u.created_at ASC")
	case domain.SortEmailAsc:
		listBuilder = listBuilder.OrderBy("u.email ASC")
	default:
		listBuilder = listBuilder.OrderBy("u.created_at DESC")
	}

	listBuilder = listBuilder.
		Limit(uint64(filter.Size)).
		Offset(uint64(filter.Page * filter.Size))

	sql, args, err := listBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build ListUsers query: %w", err)
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("execute ListUsers query: %w", err)
	}
	defer rows.Close()

	var users []domain.UserWithRoles
	for rows.Next() {
		var u domain.UserWithRoles
		err = rows.Scan(&u.ID, &u.Email, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt, &u.Roles)
		if err != nil {
			return nil, 0, fmt.Errorf("scan ListUsers row: %w", err)
		}
		users = append(users, u)
	}
	if rows.Err() != nil {
		return nil, 0, fmt.Errorf("iterate ListUsers rows: %w", rows.Err())
	}

	total, err := r.countUsers(ctx, conds)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func adminUserFilterConds(filter domain.AdminUserFilter) sq.And {
	conds := sq.And{}
	if filter.Query != "" {
		conds = append(conds, sq.ILike{"u.email": "%" + filter.Query + "%"})
	}
	if filter.Role != "" {
		conds = append(conds, sq.Expr(
			"EXISTS (SELECT 1 FROM user_roles ur2 JOIN roles r2 ON r2.id = ur2.role_id "+
				"WHERE ur2.user_id = u.id AND r2.name = ?)",
			filter.Role,
		))
	}
	if filter.EmailVerified != nil {
		conds = append(conds, sq.Eq{"u.email_verified": *filter.EmailVerified})
	}
	return conds
}

func (r *ORMRepository) countUsers(ctx context.Context, conds sq.And) (int64, error) {
	countBuilder := psql.Select("COUNT(*)").From("users u")
	if len(conds) > 0 {
		countBuilder = countBuilder.Where(conds)
	}

	sql, args, err := countBuilder.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count users query: %w", err)
	}

	var total int64
	err = r.pool.QueryRow(ctx, sql, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("execute count users query: %w", err)
	}
	return total, nil
}

func (r *ORMRepository) GetUserWithRolesByID(ctx context.Context, userID uuid.UUID) (domain.UserWithRoles, error) {
	sql, args, err := psql.
		Select("u.id", "u.email", "u.email_verified", "u.created_at", "u.updated_at", rolesAggColumn).
		From("users u").
		LeftJoin("user_roles ur ON ur.user_id = u.id").
		LeftJoin("roles r ON r.id = ur.role_id").
		Where(sq.Eq{"u.id": userID}).
		GroupBy("u.id").
		ToSql()
	if err != nil {
		return domain.UserWithRoles{}, fmt.Errorf("build GetUserWithRolesByID query: %w", err)
	}

	var u domain.UserWithRoles
	err = r.pool.QueryRow(ctx, sql, args...).
		Scan(&u.ID, &u.Email, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt, &u.Roles)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserWithRoles{}, domain.ErrNotFound
		}
		return domain.UserWithRoles{}, fmt.Errorf("execute GetUserWithRolesByID query: %w", err)
	}

	return u, nil
}

func (r *ORMRepository) RemoveRole(ctx context.Context, userID uuid.UUID, roleName string) (bool, error) {
	sql, args, err := psql.
		Delete("user_roles").
		Where(sq.Expr(
			"user_id = ? AND role_id = (SELECT id FROM roles WHERE name = ?)",
			userID, roleName,
		)).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build RemoveRole query: %w", err)
	}

	tag, err := r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return false, fmt.Errorf("execute RemoveRole query: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

func (r *ORMRepository) CountUsersByRole(ctx context.Context, roleName string) (int64, error) {
	sql, args, err := psql.
		Select("COUNT(DISTINCT ur.user_id)").
		From("user_roles ur").
		Join("roles r ON r.id = ur.role_id").
		Where(sq.Eq{"r.name": roleName}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build CountUsersByRole query: %w", err)
	}

	var count int64
	err = r.pool.QueryRow(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("execute CountUsersByRole query: %w", err)
	}
	return count, nil
}

package ormrepository

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	return roles, rows.Err()
}

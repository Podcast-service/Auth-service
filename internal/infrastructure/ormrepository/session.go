package ormrepository

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

func (r *ORMRepository) CreateRefreshToken(ctx context.Context, token domain.RefreshToken) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Insert("refresh_tokens").
		Columns("id", "user_id", "token_hash", "device_name",
			"ip_address", "user_agent", "revoked", "expires_at").
		Values(token.ID, token.UserID, token.TokenHash, token.DeviceName,
			token.IPAddress, token.UserAgent, token.Revoked, token.ExpiresAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build CreateRefreshToken query: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("execute CreateRefreshToken query: %w", err)
	}
	return nil
}

func (r *ORMRepository) GetRefreshToken(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("id", "user_id", "token_hash", "device_name",
			"ip_address", "user_agent", "revoked", "expires_at", "created_at").
		From("refresh_tokens").
		Where(sq.Eq{"token_hash": tokenHash}).
		ToSql()
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("build GetRefreshToken query: %w", err)
	}

	var token domain.RefreshToken
	var ipAddr netip.Addr
	row := r.pool.QueryRow(ctx, sql, args...)
	err = row.Scan(&token.ID, &token.UserID, &token.TokenHash, &token.DeviceName,
		&ipAddr, &token.UserAgent, &token.Revoked, &token.ExpiresAt, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RefreshToken{}, domain.ErrNotFound
		}
		return domain.RefreshToken{}, fmt.Errorf("execute GetRefreshToken query: %w", err)
	}

	if ipAddr.IsValid() {
		token.IPAddress = ipAddr.String()
	}

	return token, nil
}

func (r *ORMRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("refresh_tokens").
		Set("revoked", true).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build RevokeRefreshToken query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute RevokeRefreshToken query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("refresh_tokens").
		Set("revoked", true).
		Where(sq.Eq{"user_id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build RevokeAllUserTokens query: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute RevokeAllUserTokens query: %w", err)
	}
	return nil
}

// UpdateLastUsed не понятно, когда нужно обновлять это поле, пусть функция останется
func (r *ORMRepository) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("refresh_tokens").
		Set("last_used_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": tokenID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build UpdateLastUsed query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute UpdateLastUsed query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]domain.Device, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("id", "device_name", "ip_address", "user_agent", "last_used_at", "created_at").
		From("refresh_tokens").
		Where(sq.Eq{"user_id": userID, "revoked": false}).
		Where("expires_at > NOW()").
		OrderBy("last_used_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build GetUserDevices query: %w", err)
	}

	var rows pgx.Rows

	rows, err = r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("execute GetUserDevices query: %w", err)
	}
	defer rows.Close()

	var devices []domain.Device
	for rows.Next() {
		var device domain.Device
		var ipAddr netip.Addr
		err = rows.Scan(&device.RefreshTokenID, &device.DeviceName, &ipAddr,
			&device.UserAgent, &device.LastUsedAt, &device.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan GetUserDevices row: %w", err)
		}
		if ipAddr.IsValid() {
			device.IPAddress = ipAddr.String()
		}
		devices = append(devices, device)
	}

	if rows.Err() != nil {
		return devices, fmt.Errorf("get user devices: %w", rows.Err())
	}
	return devices, nil
}

func (r *ORMRepository) DeleteExpiredTokens(ctx context.Context) error {
	sql, args, err := psql.
		Delete("refresh_tokens").
		Where("expires_at < NOW()").
		ToSql()
	if err != nil {
		return fmt.Errorf("build DeleteExpiredTokens query: %w", err)
	}
	if _, err = r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("execute DeleteExpiredTokens query: %w", err)
	}
	return nil
}

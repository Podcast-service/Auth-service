package ormrepository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

var (
	psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
)

// у меня две функции и учавствуют в транзакции(используют существующий пул),
// и используются как само достаточные(создают новый пул)
// поэтому я ввёл интерфейс  который реализуют и pgxpool.Pool(создающий новый пул)
// и pgx.Tx(использующий, существующий пул)
type queryRunner interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type ORMRepository struct {
	pool *pgxpool.Pool
}

func NewORMRepository(pool *pgxpool.Pool) *ORMRepository {
	return &ORMRepository{pool: pool}
}

func (r *ORMRepository) createUser(ctx context.Context, tx pgx.Tx, email, passwordHash string) (uuid.UUID, error) {
	var err error
	var sql string
	var args []interface{}
	id := uuid.New()
	sql, args, err = psql.
		Insert("users").
		Columns("id", "email", "password_hash").
		Values(id, email, passwordHash).
		ToSql()
	if err != nil {
		return uuid.Nil, fmt.Errorf("build CreateUser query: %w", err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, domain.ErrAlreadyExists
		}
		return uuid.Nil, fmt.Errorf("execute CreateUser query: %w", err)
	}

	return id, nil
}

func (r *ORMRepository) assignRole(ctx context.Context, db queryRunner, userID uuid.UUID, roleName string) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("id").
		From("roles").
		Where(sq.Eq{"name": roleName}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build assignRole query: %w", err)
	}

	var roleID uuid.UUID
	row := db.QueryRow(ctx, sql, args...)
	err = row.Scan(&roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("role %s not found: %w", roleName, domain.ErrNotFound)
		}
		return fmt.Errorf("execute assignRole query: %w", err)
	}

	sql, args, err = psql.
		Insert("user_roles").
		Columns("user_id", "role_id").
		Values(userID, roleID).
		ToSql()
	if err != nil {
		return fmt.Errorf("build assignRole insert query: %w", err)
	}

	_, err = db.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("execute assignRole insert query: %w", err)
	}
	return nil
}

func (r *ORMRepository) verifyEmail(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("users").
		Set("email_verified", true).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build verifyEmail query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute verifyEmail query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) markPasswordResetTokenUsed(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("password_reset_tokens").
		Set("used", true).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build MarkPasswordResetTokenUsed query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute MarkPasswordResetTokenUsed query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) markEmailVerifyTokenUsed(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("email_verification_tokens").
		Set("used", true).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build MarkEmailVerifyTokenUsed query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute MarkEmailVerifyTokenUsed query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) updatePasswordHash(ctx context.Context, tx pgx.Tx, userID uuid.UUID, newHash string) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Update("users").
		Set("password_hash", newHash).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build updatePasswordHash query: %w", err)
	}

	var cmdTag pgconn.CommandTag

	cmdTag, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("execute updatePasswordHash query: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ORMRepository) createEmailVerifyToken(ctx context.Context, db queryRunner, token domain.EmailVerifyToken) error {
	sql, args, err := psql.
		Insert("email_verification_tokens").
		Columns("id", "user_id", "code", "expires_at").
		Values(token.ID, token.UserID, token.Code, token.ExpiresAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build CreateEmailVerifyToken query: %w", err)
	}

	_, err = db.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("execute CreateEmailVerifyCode query: %w", err)
	}
	return nil
}

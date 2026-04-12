package ormrepository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Podcast-service/Auth-service/internal/domain"
)

func (r *ORMRepository) RegisterUser(ctx context.Context, email, passwordHash string, verifyToken domain.EmailVerifyToken) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		rbErr := tx.Rollback(ctx)
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Printf("rollback transaction: %v", rbErr)
		}
	}()

	var userID uuid.UUID
	userID, err = r.createUser(ctx, tx, email, passwordHash)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}

	err = r.assignRole(ctx, tx, userID, "user")
	if err != nil {
		return uuid.Nil, fmt.Errorf("assign role: %w", err)
	}

	verifyToken.UserID = userID
	err = r.createEmailVerifyToken(ctx, tx, verifyToken)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create email verify token: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("commit transaction: %w", err)
	}

	return userID, nil
}

func (r *ORMRepository) ConfirmEmail(ctx context.Context, token domain.EmailVerifyToken) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		rbErr := tx.Rollback(ctx)
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Printf("rollback transaction: %v", rbErr)
		}
	}()

	err = r.verifyEmail(ctx, tx, token.UserID)
	if err != nil {
		return fmt.Errorf("verify email: %w", err)
	}

	err = r.markEmailVerifyTokenUsed(ctx, tx, token.ID)
	if err != nil {
		return fmt.Errorf("mark email verify token used: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *ORMRepository) ResetPassword(ctx context.Context, token domain.PasswordResetToken, newPasswordHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		rbErr := tx.Rollback(ctx)
		if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Printf("rollback transaction: %v", rbErr)
		}
	}()

	err = r.updatePasswordHash(ctx, tx, token.UserID, newPasswordHash)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}

	err = r.markPasswordResetTokenUsed(ctx, tx, token.ID)
	if err != nil {
		return fmt.Errorf("mark password reset token used: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *ORMRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("id", "email", "password_hash", "email_verified", "created_at", "updated_at").
		From("users").
		Where(sq.Eq{"email": email}).
		ToSql()
	if err != nil {
		return domain.User{}, fmt.Errorf("build GetUserByEmail query: %w", err)
	}

	var user domain.User

	row := r.pool.QueryRow(ctx, sql, args...)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash,
		&user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("execute GetUserByEmail query: %w", err)
	}

	return user, nil
}

func (r *ORMRepository) GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	sql, args, err := psql.
		Select("id", "email", "password_hash", "email_verified", "created_at", "updated_at").
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return domain.User{}, fmt.Errorf("build GetUserByID query: %w", err)
	}

	var user domain.User

	row := r.pool.QueryRow(ctx, sql, args...)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash,
		&user.EmailVerified, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("execute GetUserByID query: %w", err)
	}

	return user, nil
}

func (r *ORMRepository) GetEmailVerifyToken(ctx context.Context, email string, code string) (domain.EmailVerifyToken, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("evt.id", "evt.user_id", "evt.code", "evt.expires_at", "evt.used").
		From("email_verification_tokens evt").
		Join("users u ON u.id = evt.user_id").
		Where(sq.Eq{"u.email": email, "evt.code": code}).
		ToSql()
	if err != nil {
		return domain.EmailVerifyToken{}, fmt.Errorf("build GetEmailVerifyToken query: %w", err)
	}

	var token domain.EmailVerifyToken
	row := r.pool.QueryRow(ctx, sql, args...)
	err = row.Scan(&token.ID, &token.UserID, &token.Code, &token.ExpiresAt, &token.Used)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.EmailVerifyToken{}, domain.ErrNotFound
		}
		return domain.EmailVerifyToken{}, fmt.Errorf("execute GetEmailVerifyToken query: %w", err)
	}

	return token, nil
}

func (r *ORMRepository) CreatePasswordResetToken(ctx context.Context, token domain.PasswordResetToken) error {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Insert("password_reset_tokens").
		Columns("id", "user_id", "code", "expires_at").
		Values(token.ID, token.UserID, token.Code, token.ExpiresAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("build CreatePasswordResetToken query: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyExists
		}
		return fmt.Errorf("execute CreatePasswordResetToken query: %w", err)
	}
	return nil
}

func (r *ORMRepository) GetPasswordResetToken(ctx context.Context, email string, code string) (domain.PasswordResetToken, error) {
	var err error
	var sql string
	var args []interface{}
	sql, args, err = psql.
		Select("prt.id", "prt.user_id", "prt.code", "prt.expires_at", "prt.used", "prt.created_at").
		From("password_reset_tokens prt").
		Join("users u ON u.id = prt.user_id").
		Where(sq.Eq{"u.email": email, "prt.code": code}).
		ToSql()
	if err != nil {
		return domain.PasswordResetToken{}, fmt.Errorf("build GetPasswordResetToken query: %w", err)
	}
	row := r.pool.QueryRow(ctx, sql, args...)

	var token domain.PasswordResetToken

	err = row.Scan(&token.ID, &token.UserID, &token.Code, &token.ExpiresAt, &token.Used, &token.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PasswordResetToken{}, domain.ErrNotFound
		}
		return domain.PasswordResetToken{}, fmt.Errorf("execute GetPasswordResetToken query: %w", err)
	}

	return token, nil
}

func (r *ORMRepository) CreateEmailVerifyToken(ctx context.Context, token domain.EmailVerifyToken) error {
	err := r.createEmailVerifyToken(ctx, r.pool, token)
	if err != nil {
		return fmt.Errorf("create email verify token: %w", err)
	}
	return nil
}

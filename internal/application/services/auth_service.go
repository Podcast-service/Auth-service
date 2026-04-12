package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Podcast-service/Auth-service/internal/application"
	"github.com/Podcast-service/Auth-service/internal/application/generator"
	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

const (
	verificationTokenTTL  = 24 * time.Hour
	passwordResetTokenTTL = 1 * time.Hour
)

type AuthServices interface {
	Register(ctx context.Context, req dto.RegisterRequest) (dto.User, error)
	VerifyEmail(ctx context.Context, req dto.VerifyEmailRequest) (dto.TokenResponse, error)
	ResendVerification(ctx context.Context, req dto.ResendVerificationRequest) error
	Login(ctx context.Context, req dto.LoginRequest, ipAddress, userAgent string) (dto.TokenResponse, error)
	Refresh(ctx context.Context, req dto.RefreshTokenRequest) (dto.TokenResponse, error)
	RequestPasswordReset(ctx context.Context, req dto.PasswordResetRequest) error
	ConfirmPasswordReset(ctx context.Context, req dto.PasswordResetConfirmRequest) error
}

type authService struct {
	authRepo        application.AuthRepository
	sessionRepo     application.SessionRepository
	userRepo        application.UserRepository
	sender          Sender
	jwtManager      *access.Manager
	refreshTokenTTL time.Duration
}

func NewAuthService(
	authRepo application.AuthRepository,
	sessionRepo application.SessionRepository,
	userRepo application.UserRepository,
	sender Sender,
	jwtManager *access.Manager,
	refreshTokenTTL time.Duration,
) AuthServices {
	return &authService{
		authRepo:        authRepo,
		sessionRepo:     sessionRepo,
		userRepo:        userRepo,
		sender:          sender,
		jwtManager:      jwtManager,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (a authService) Register(ctx context.Context, req dto.RegisterRequest) (dto.User, error) {
	log := logging.FromContext(ctx)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("hashing password failed",
			slog.String("error", err.Error()),
		)
		return dto.User{}, fmt.Errorf("hash password: %w", err)
	}

	var code string
	code, err = generator.GenerateCode()
	if err != nil {
		return dto.User{}, fmt.Errorf("generate refresh token: %w", err)
	}

	verifyToken := domain.EmailVerifyToken{
		ID:        uuid.New(),
		Code:      code,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	}

	var userID uuid.UUID
	userID, err = a.authRepo.RegisterUser(ctx, req.Email, string(passwordHash), verifyToken)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			log.Warn("user already exists",
				slog.String("email", req.Email),
			)
			return dto.User{}, domain.ErrAlreadyExists
		}
		return dto.User{}, fmt.Errorf("register user: %w", err)
	}

	a.sendEmailVerifyMessage(ctx, code, req.Email)

	return dto.User{
		ID:    userID,
		Email: req.Email,
	}, nil

}

func (a authService) VerifyEmail(ctx context.Context, req dto.VerifyEmailRequest) (dto.TokenResponse, error) {
	log := logging.FromContext(ctx)

	token, err := a.authRepo.GetEmailVerifyToken(ctx, req.Email, req.Code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("email verify token not recorded",
				slog.String("email", req.Email),
				slog.String("code", req.Code),
			)
			return dto.TokenResponse{}, domain.ErrInvalidCredentials
		}
		return dto.TokenResponse{}, fmt.Errorf("get email verify token: %w", err)
	}

	if token.Used {
		log.Warn("email verify code already used",
			slog.String("token_id", token.ID.String()),
			slog.String("user_id", token.UserID.String()),
		)
		return dto.TokenResponse{}, domain.ErrAlreadyCodeUsed
	}

	if time.Now().After(token.ExpiresAt) {
		log.Warn("email verify token expired",
			slog.String("token_id", token.ID.String()),
			slog.String("user_id", token.UserID.String()),
		)
		return dto.TokenResponse{}, domain.ErrTokenExpired
	}

	err = a.authRepo.ConfirmEmail(ctx, token)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("confirm email: %w", err)
	}

	var user domain.User
	user, err = a.authRepo.GetUserByID(ctx, token.UserID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("get user by id: %w", err)
	}
	var roles []string
	roles, err = a.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("get user roles: %w", err)
	}

	var resp dto.TokenResponse
	resp, err = a.issueTokenPair(ctx, user, roles, "Initial Device", "System", "")
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("issue token pair: %w", err)
	}

	log.Info("email verify token confirmed",
		slog.String("token_id", token.ID.String()),
		slog.String("user_id", token.UserID.String()),
	)

	return resp, nil
}

func (a authService) ResendVerification(ctx context.Context, req dto.ResendVerificationRequest) error {
	log := logging.FromContext(ctx)

	user, err := a.authRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("user not found for password resend",
				slog.String("email", req.Email),
			)
			return nil
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	if user.EmailVerified {
		log.Warn("email already verified",
			slog.String("email", req.Email),
		)
		return nil
	}

	var code string
	code, err = generator.GenerateCode()
	if err != nil {
		return fmt.Errorf("generate verification code: %w", err)
	}

	verifyToken := domain.EmailVerifyToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Code:      code,
		ExpiresAt: time.Now().Add(verificationTokenTTL),
	}
	err = a.authRepo.CreateEmailVerifyToken(ctx, verifyToken)
	if err != nil {
		return fmt.Errorf("create email verify token: %w", err)
	}

	a.sendEmailVerifyMessage(ctx, code, req.Email)

	return nil
}

func (a authService) Login(ctx context.Context, req dto.LoginRequest, ipAddress, userAgent string) (dto.TokenResponse, error) {
	log := logging.FromContext(ctx)

	user, err := a.authRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("user not found",
				slog.String("email", req.Email),
			)
			return dto.TokenResponse{}, domain.ErrInvalidCredentials
		}
		return dto.TokenResponse{}, fmt.Errorf("get user by email: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		log.Warn("invalid password",
			slog.String("email", req.Email),
		)
		return dto.TokenResponse{}, domain.ErrInvalidCredentials
	}
	if !user.EmailVerified {
		log.Warn("email not verified",
			slog.String("email", req.Email),
		)
		go func() {
			if err = a.ResendVerification(context.WithoutCancel(ctx), dto.ResendVerificationRequest{Email: req.Email}); err != nil {
				slog.Error("failed to resend verification email on login",
					slog.String("email", req.Email),
					slog.String("error", err.Error()),
				)
			}
		}()
		return dto.TokenResponse{}, domain.ErrEmailNotVerified
	}

	var roles []string
	roles, err = a.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("get user roles: %w", err)
	}

	var resp dto.TokenResponse
	resp, err = a.issueTokenPair(ctx, user, roles, req.DeviceName, userAgent, ipAddress)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("issue token pair: %w", err)
	}

	log.Info("user logged in",
		slog.String("user_id", user.ID.String()),
		slog.String("device_name", req.DeviceName),
	)

	return resp, nil
}

func (a authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (dto.TokenResponse, error) {
	log := logging.FromContext(ctx)
	tokenHash := generator.HashToken(req.RefreshToken)
	storedToken, err := a.sessionRepo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("refresh token not found",
				slog.String("token_hash", tokenHash),
			)
			return dto.TokenResponse{}, domain.ErrInvalidCredentials
		}
		return dto.TokenResponse{}, fmt.Errorf("get refresh token: %w", err)
	}

	if time.Now().After(storedToken.ExpiresAt) {
		log.Warn("refresh token expired",
			slog.String("token_id", storedToken.ID.String()),
			slog.String("user_id", storedToken.UserID.String()),
		)
		return dto.TokenResponse{}, domain.ErrTokenExpired
	}

	if storedToken.Revoked {
		log.Warn("refresh token revoked",
			slog.String("token_id", storedToken.ID.String()),
			slog.String("user_id", storedToken.UserID.String()),
		)
		return dto.TokenResponse{}, domain.ErrTokenRevoked
	}

	err = a.sessionRepo.RevokeRefreshToken(ctx, storedToken.ID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("revoke refresh token: %w", err)
	}

	var user domain.User
	user, err = a.authRepo.GetUserByID(ctx, storedToken.UserID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("get user by id: %w", err)
	}

	var roles []string
	roles, err = a.userRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("get user roles: %w", err)
	}

	var resp dto.TokenResponse
	resp, err = a.issueTokenPair(ctx, user, roles, storedToken.DeviceName, storedToken.UserAgent, storedToken.IPAddress)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("issue token pair: %w", err)
	}

	log.Info("token refreshed",
		slog.String("user_id", user.ID.String()),
		slog.String("refresh_token_id", storedToken.ID.String()),
	)

	return resp, nil
}

func (a authService) RequestPasswordReset(ctx context.Context, req dto.PasswordResetRequest) error {
	log := logging.FromContext(ctx)

	user, err := a.authRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("user not found",
				slog.String("email", req.Email),
			)
			return nil
		}
		return fmt.Errorf("get user by email: %w", err)
	}

	var code string
	code, err = generator.GenerateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	resetToken := domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Code:      code,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	}
	err = a.authRepo.CreatePasswordResetToken(ctx, resetToken)
	if err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}

	err = a.sender.SendMessage(ctx, dto.PasswordResetMessage{
		Type:      "PASSWORD_RESET",
		Email:     req.Email,
		ResetCode: code,
	})
	if err != nil {
		slog.Error("send password reset message",
			slog.String("error", err.Error()),
			slog.String("email", req.Email),
		)
	}

	log.Info("password reset message",
		slog.String("user_id", user.ID.String()),
	)

	return nil
}

func (a authService) ConfirmPasswordReset(ctx context.Context, req dto.PasswordResetConfirmRequest) error {
	log := logging.FromContext(ctx)
	token, err := a.authRepo.GetPasswordResetToken(ctx, req.Email, req.Code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("password reset token not found",
				slog.String("email", req.Email),
				slog.String("code", req.Code),
			)
			return domain.ErrInvalidCredentials
		}
		return fmt.Errorf("get password reset token: %w", err)
	}

	if time.Now().After(token.ExpiresAt) {
		log.Warn("password reset token expired",
			slog.String("token_id", token.ID.String()),
			slog.String("user_id", token.UserID.String()),
		)
		return domain.ErrTokenExpired
	}

	if token.Used {
		log.Warn("password reset code already used",
			slog.String("token_id", token.ID.String()),
			slog.String("user_id", token.UserID.String()),
		)
		return domain.ErrAlreadyCodeUsed
	}

	var newPasswordHash []byte
	newPasswordHash, err = bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	err = a.authRepo.ResetPassword(ctx, token, string(newPasswordHash))
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}

	log.Info("password reset confirmed",
		slog.String("token_id", token.ID.String()),
		slog.String("user_id", token.UserID.String()),
	)

	return nil
}

func (a authService) issueTokenPair(ctx context.Context, user domain.User, roles []string, deviceName, userAgent, ipAddress string) (dto.TokenResponse, error) {
	accessToken, err := a.jwtManager.GenerateAccessToken(user.ID, user.Email, roles)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("generate access token: %w", err)
	}
	var raw string
	var hash string
	raw, hash, err = generator.GenerateToken()
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := domain.RefreshToken{
		ID:         uuid.New(),
		UserID:     user.ID,
		TokenHash:  hash,
		DeviceName: deviceName,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		ExpiresAt:  time.Now().Add(a.refreshTokenTTL),
	}
	err = a.sessionRepo.CreateRefreshToken(ctx, refreshToken)
	if err != nil {
		return dto.TokenResponse{}, fmt.Errorf("create refresh token: %w", err)
	}

	return dto.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: raw,
		ExpiresIn:    int64(a.jwtManager.AccessTokenTTL.Seconds()),
	}, nil
}

func (a authService) sendEmailVerifyMessage(ctx context.Context, code, email string) {
	err := a.sender.SendMessage(ctx, dto.EmailVerifyMessage{
		Type:       "EMAIL_VERIFY",
		Email:      email,
		VerifyCode: code,
	})
	if err != nil {
		slog.Error("send email verify message",
			slog.String("error", err.Error()),
			slog.String("email", email),
		)
	}
}

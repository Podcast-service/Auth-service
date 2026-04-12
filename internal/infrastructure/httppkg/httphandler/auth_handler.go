package httphandler

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httputils"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

type AuthHandler struct {
	svc services.AuthServices
}

func NewAuthHandler(svc services.AuthServices) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.RegisterRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode register request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		log.Warn("email or password is empty")
		httputils.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var user dto.User
	user, err = h.svc.Register(r.Context(), req)
	if err != nil {
		log.Warn("failed to register user",
			slog.String("error", err.Error()),
			slog.String("email", req.Email),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("user registered successfully",
		slog.String("user_id", user.ID.String()),
		slog.String("email", user.Email),
	)

	httputils.WriteJSON(w, http.StatusCreated, user)
}

func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.VerifyEmailRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode verify email request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" {
		log.Warn("verification code is empty")
		httputils.WriteError(w, http.StatusBadRequest, "code is required")
		return
	}

	var resp dto.TokenResponse
	resp, err = h.svc.VerifyEmail(r.Context(), req)
	if err != nil {
		log.Warn("failed to verify email",
			slog.String("error", err.Error()),
			slog.String("code", req.Code),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("verified email successfully",
		slog.String("email", req.Email),
	)

	httputils.WriteJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode login request",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		log.Warn("email or password is empty for login request")
		httputils.WriteError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var ipAddress string
	ipAddress, _, err = net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ipAddress = r.RemoteAddr
	}
	userAgent := r.Header.Get("User-Agent")

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.SplitN(forwarded, ",", 2)
		ipAddress = strings.TrimSpace(parts[0])
	}

	var token dto.TokenResponse
	token, err = h.svc.Login(r.Context(), req, ipAddress, userAgent)
	if err != nil {
		log.Warn("failed to login",
			slog.String("email", req.Email),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("logged in successfully",
		slog.String("email", req.Email),
	)

	httputils.WriteJSON(w, http.StatusOK, token)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.RefreshTokenRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode refresh token request",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		log.Warn("refresh token is empty for refresh token request")
		httputils.WriteError(w, http.StatusBadRequest, "refresh token is required")
		return
	}

	var token dto.TokenResponse
	token, err = h.svc.Refresh(r.Context(), req)
	if err != nil {
		log.Warn("failed to refresh token",
			slog.String("refresh_token", req.RefreshToken),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("tokens refreshed successfully",
		slog.String("refresh_token", req.RefreshToken),
	)

	httputils.WriteJSON(w, http.StatusOK, token)
}

func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.PasswordResetRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode password reset request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		log.Warn("email is empty for password reset request")
		httputils.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}

	_ = h.svc.RequestPasswordReset(r.Context(), req)

	log.Info("password reset request successfully",
		slog.String("email", req.Email),
	)

	httputils.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "if this email exists, a reset link has been sent"})
}

func (h *AuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.PasswordResetConfirmRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode password reset confirm request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" || req.NewPassword == "" {
		log.Warn("code or new password is empty for password reset confirm request")
		httputils.WriteError(w, http.StatusBadRequest, "code and new_password are required")
		return
	}

	err = h.svc.ConfirmPasswordReset(r.Context(), req)
	if err != nil {
		log.Warn("failed to confirm password reset",
			slog.String("error", err.Error()),
			slog.String("code", req.Code),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("password reset confirm request successfully",
		slog.String("code", req.Code),
	)

	httputils.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "password has been reset"})
}

func (h *AuthHandler) ResendVerificationEmail(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	var req dto.ResendVerificationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode resend verification email request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		log.Warn("email is empty for resend verification email request")
		httputils.WriteError(w, http.StatusBadRequest, "email is required")
		return
	}

	err = h.svc.ResendVerification(r.Context(), req)
	if err != nil {
		log.Warn("failed to resend verification email",
			slog.String("error", err.Error()),
			slog.String("email", req.Email),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("resend verification email successfully",
		slog.String("email", req.Email),
	)

	httputils.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "if this email exists and is not verified, a new verification email has been sent"})
}

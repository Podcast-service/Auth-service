package httphandler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httputils"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

type SessionHandler struct {
	svc services.SessionService
}

func NewSessionHandler(svc services.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

func (h *SessionHandler) Devices(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	userID, ok := httputils.GetUserIDFromContext(r)
	if !ok {
		log.Warn("failed to get user ID from context for getting devices")
		httputils.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	devices, err := h.svc.Devices(r.Context(), userID)
	if err != nil {
		log.Warn("failed to get devices",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("devices received successfully",
		slog.String("user_id", userID.String()),
		slog.Int("devices_count", len(devices)),
	)

	httputils.WriteJSON(w, http.StatusOK, devices)
}

func (h *SessionHandler) Logout(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	userID, ok := httputils.GetUserIDFromContext(r)
	if !ok {
		log.Warn("failed to get user ID from context for logout request")
		httputils.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var token dto.LogoutRequest
	err := json.NewDecoder(r.Body).Decode(&token)
	if err != nil {
		log.Warn("failed to decode logout request",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if token.RefreshToken == "" {
		log.Warn("refresh token is empty for logout request",
			slog.String("user_id", userID.String()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	err = h.svc.Logout(r.Context(), userID, token)
	if err != nil {
		log.Warn("failed to logout",
			slog.String("user_id", userID.String()),
			slog.String("refresh_token", token.RefreshToken),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("logged out successfully",
		slog.String("user_id", userID.String()),
		slog.String("refresh_token", token.RefreshToken),
	)

	httputils.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged out"})
}

func (h *SessionHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	userID, ok := httputils.GetUserIDFromContext(r)
	if !ok {
		log.Warn("failed to get user ID from context for logout all request")
		httputils.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.svc.LogoutAll(r.Context(), userID)
	if err != nil {
		log.Warn("failed to logout from all devices",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("logged out from all devices successfully",
		slog.String("user_id", userID.String()),
	)

	httputils.WriteJSON(w, http.StatusOK, dto.MessageResponse{Message: "logged out from all devices"})
}

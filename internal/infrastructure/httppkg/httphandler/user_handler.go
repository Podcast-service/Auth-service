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

type UserHandler struct {
	svc services.UserService
}

func NewUserHandler(svc services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Roles(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	userID, ok := httputils.GetUserIDFromContext(r)
	if !ok {
		log.Warn("failed to get user ID from context for getting roles")
		httputils.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roles, err := h.svc.Roles(r.Context(), userID)
	if err != nil {
		log.Warn("failed to get user roles",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()))
		httputils.MapError(w, err)
		return
	}

	log.Info("user roles received successfully",
		slog.String("user_id", userID.String()),
		slog.Int("roles_count", len(roles.Roles)),
	)

	httputils.WriteJSON(w, http.StatusOK, roles)
}

func (h *UserHandler) UpdateRoles(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	userID, ok := httputils.GetUserIDFromContext(r)
	if !ok {
		log.Warn("failed to get user ID from context for updating roles")
		httputils.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.UpdateRolesRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Warn("failed to decode role name request body",
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RoleName == "" {
		log.Warn("role name is empty for updating roles")
		httputils.WriteError(w, http.StatusBadRequest, "role_name is required")
		return
	}

	var resp dto.AccessTokenResponse
	resp, err = h.svc.UpdateRoles(r.Context(), userID, req)
	if err != nil {
		log.Warn("failed to update user roles",
			slog.String("user_id", userID.String()),
			slog.String("role_name", req.RoleName),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("user roles updated successfully",
		slog.String("user_id", userID.String()),
		slog.String("role_name", req.RoleName),
	)

	httputils.WriteJSON(w, http.StatusOK, resp)
}

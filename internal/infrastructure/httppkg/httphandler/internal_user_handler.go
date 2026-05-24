package httphandler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httputils"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

type InternalUserHandler struct {
	svc services.UserService
}

func NewInternalUserHandler(svc services.UserService) *InternalUserHandler {
	return &InternalUserHandler{svc: svc}
}

func (h *InternalUserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	userIDStr := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Warn("invalid user_id in internal get user request",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}

	var user dto.User
	user, err = h.svc.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("user not found for internal get user request",
				slog.String("user_id", userID.String()),
			)
			httputils.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Warn("failed to get user by id for internal request",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	log.Info("internal user fetched successfully",
		slog.String("user_id", user.ID.String()),
	)

	httputils.WriteJSON(w, http.StatusOK, user)
}

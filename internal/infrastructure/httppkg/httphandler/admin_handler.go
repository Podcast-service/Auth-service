package httphandler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/application/services"
	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httputils"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/logging"
)

type AdminHandler struct {
	svc services.AdminService
}

func NewAdminHandler(svc services.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	filter, err := parseAdminUserFilter(r)
	if err != nil {
		log.Warn("invalid admin list users query", slog.String("error", err.Error()))
		httputils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.ListUsers(r.Context(), filter)
	if err != nil {
		log.Warn("failed to list users", slog.String("error", err.Error()))
		httputils.MapError(w, err)
		return
	}

	httputils.WriteJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	userID, ok := parseUserIDParam(w, r, log)
	if !ok {
		return
	}

	user, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("admin get user: not found", slog.String("user_id", userID.String()))
			httputils.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Warn("failed to get user", slog.String("user_id", userID.String()), slog.String("error", err.Error()))
		httputils.MapError(w, err)
		return
	}

	httputils.WriteJSON(w, http.StatusOK, user)
}

func (h *AdminHandler) AddRole(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	userID, ok := parseUserIDParam(w, r, log)
	if !ok {
		return
	}

	var req dto.AdminRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("failed to decode admin add role body", slog.String("error", err.Error()))
		httputils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RoleName == "" {
		log.Warn("role name is empty for admin add role")
		httputils.WriteError(w, http.StatusBadRequest, "role_name is required")
		return
	}

	resp, err := h.svc.AddRole(r.Context(), userID, req.RoleName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("admin add role: user not found", slog.String("user_id", userID.String()))
			httputils.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Warn("failed to add role",
			slog.String("user_id", userID.String()),
			slog.String("role_name", req.RoleName),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	httputils.WriteJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) RemoveAdminRole(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())

	userID, ok := parseUserIDParam(w, r, log)
	if !ok {
		return
	}

	resp, err := h.svc.RemoveAdminRole(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Warn("admin remove role: user not found", slog.String("user_id", userID.String()))
			httputils.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Warn("failed to remove admin role",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		httputils.MapError(w, err)
		return
	}

	httputils.WriteJSON(w, http.StatusOK, resp)
}

func parseUserIDParam(w http.ResponseWriter, r *http.Request, log *slog.Logger) (uuid.UUID, bool) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Warn("invalid user_id in admin request",
			slog.String("user_id", userIDStr),
			slog.String("error", err.Error()),
		)
		httputils.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return uuid.Nil, false
	}
	return userID, true
}

func parseAdminUserFilter(r *http.Request) (domain.AdminUserFilter, error) {
	q := r.URL.Query()
	filter := domain.AdminUserFilter{
		Query: q.Get("q"),
		Page:  domain.DefaultPageNo,
		Size:  domain.DefaultSize,
		Sort:  domain.DefaultSort,
	}

	if role := q.Get("role"); role != "" {
		if !domain.IsAllowedRole(role) {
			return domain.AdminUserFilter{}, errors.New("invalid role filter")
		}
		filter.Role = role
	}

	if ev := q.Get("email_verified"); ev != "" {
		parsed, err := strconv.ParseBool(ev)
		if err != nil {
			return domain.AdminUserFilter{}, errors.New("invalid email_verified")
		}
		filter.EmailVerified = &parsed
	}

	if page := q.Get("page"); page != "" {
		parsed, err := strconv.Atoi(page)
		if err != nil || parsed < 0 {
			return domain.AdminUserFilter{}, errors.New("invalid page")
		}
		filter.Page = parsed
	}

	if size := q.Get("size"); size != "" {
		parsed, err := strconv.Atoi(size)
		if err != nil || parsed <= 0 {
			return domain.AdminUserFilter{}, errors.New("invalid size")
		}
		filter.Size = parsed
	}

	if sort := q.Get("sort"); sort != "" {
		if !domain.IsAllowedSort(sort) {
			return domain.AdminUserFilter{}, errors.New("invalid sort")
		}
		filter.Sort = sort
	}

	return filter, nil
}

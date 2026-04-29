package httputils

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/domain"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/dto"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/authmiddleware"
)

func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		encoder := json.NewEncoder(w)
		if err := encoder.Encode(data); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	}
}

func WriteError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSON(w, statusCode, map[string]string{"error": message})
}

func MapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrAlreadyCodeUsed):
		WriteError(w, http.StatusConflict, "code already used")
	case errors.Is(err, domain.ErrInvalidCredentials):
		WriteError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, domain.ErrTokenExpired):
		WriteError(w, http.StatusUnauthorized, "token expired")
	case errors.Is(err, domain.ErrEmailNotVerified):
		WriteJSON(w, http.StatusForbidden, dto.EmailNotVerifiedResponse{
			Error:   "email_not_verified",
			Message: "Email не подтверждён. Код верификации отправлен на почту.",
		})
	case errors.Is(err, domain.ErrTokenRevoked):
		WriteError(w, http.StatusUnauthorized, "token revoked")
	case errors.Is(err, domain.ErrForbidden):
		WriteError(w, http.StatusForbidden, "token does not have access to this resource")
	case errors.Is(err, domain.ErrAlreadyExists):
		WriteError(w, http.StatusConflict, "resource already exists")

	default:
		WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func GetUserIDFromContext(r *http.Request) (uuid.UUID, bool) {
	claim, ok := authmiddleware.ClaimsFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	return claim.UserID, true
}

package access

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens"
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Roles  []string  `json:"roles"`
	jwt.RegisteredClaims
}

type Manager struct {
	secretKey      []byte
	AccessTokenTTL time.Duration
}

func NewManager(secretKey string, accessTokenTTL time.Duration) *Manager {
	return &Manager{
		secretKey:      []byte(secretKey),
		AccessTokenTTL: accessTokenTTL,
	}
}

func (m *Manager) GenerateAccessToken(userID uuid.UUID, email string, roles []string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signedToken, nil
}

func (m *Manager) ParseAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.secretKey, nil
		})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, tokens.ErrTokenExpired
		}
		return nil, tokens.ErrTokenInvalid
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, tokens.ErrTokenInvalid
	}
	return claims, nil
}

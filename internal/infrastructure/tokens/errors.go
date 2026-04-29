package tokens

import "errors"

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenMissing = errors.New("token missing")
)

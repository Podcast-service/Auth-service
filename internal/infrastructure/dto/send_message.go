package dto

type EmailVerifyMessage struct {
	Type       string `json:"type"`
	Email      string `json:"email"`
	VerifyCode string `json:"code"`
}

type PasswordResetMessage struct {
	Type      string `json:"type"`
	Email     string `json:"email"`
	ResetCode string `json:"code"`
}

type UserRegisteredMessage struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

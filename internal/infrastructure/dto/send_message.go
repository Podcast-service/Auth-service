package dto

type EmailVerifyMessage struct {
	Type       string `json:"type"`
	Email      string `json:"email"`
	VerifyCode string `json:"verify_code"`
}

type PasswordResetMessage struct {
	Type      string `json:"type"`
	Email     string `json:"email"`
	ResetCode string `json:"reset_code"`
}

type UserRegisteredMessage struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

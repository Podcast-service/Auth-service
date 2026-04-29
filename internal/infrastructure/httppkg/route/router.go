package route

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/authmiddleware"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httphandler"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

func RegisterRoutes(
	auth *httphandler.AuthHandler,
	session *httphandler.SessionHandler,
	user *httphandler.UserHandler,
	jwtManager *access.Manager,
) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", auth.Register)
		r.Post("/resend-verification", auth.ResendVerificationEmail)
		r.Post("/verify-email", auth.VerifyEmail)
		r.Post("/login", auth.Login)
		r.Post("/refresh", auth.Refresh)
		r.Route("/password-reset", func(r chi.Router) {
			r.Post("/request", auth.RequestPasswordReset)
			r.Post("/confirm", auth.ConfirmPasswordReset)
		})
		r.Group(func(r chi.Router) {
			r.Use(authmiddleware.AuthMiddleware(jwtManager))
			r.Post("/logout", session.Logout)
			r.Post("/logout_all", session.LogoutAll)
			r.Get("/devices", session.Devices)
			r.Route("/me", func(r chi.Router) {
				r.Get("/roles", user.Roles)
				r.Post("/update-roles", user.UpdateRoles)
			})

		})
	})
	return r
}

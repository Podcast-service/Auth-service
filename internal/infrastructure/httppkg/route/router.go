package route

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/authmiddleware"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/httppkg/httphandler"
	"github.com/Podcast-service/Auth-service/internal/infrastructure/tokens/access"
)

func RegisterRoutes(
	auth *httphandler.AuthHandler,
	session *httphandler.SessionHandler,
	user *httphandler.UserHandler,
	internalUser *httphandler.InternalUserHandler,
	admin *httphandler.AdminHandler,
	jwtManager *access.Manager,
) chi.Router {
	r := chi.NewRouter()
	r.Use(otelHTTPMiddleware)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
			"http://localhost:1420",
			"http://tauri.localhost",
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

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
			r.Post("/password-change", auth.ChangePassword)
			r.Post("/logout", session.Logout)
			r.Post("/logout_all", session.LogoutAll)
			r.Get("/devices", session.Devices)
			r.Route("/me", func(r chi.Router) {
				r.Get("/roles", user.Roles)
				r.Post("/update-roles", user.UpdateRoles)
			})

		})
		r.Route("/internal", func(r chi.Router) {
			r.Get("/users/{user_id}", internalUser.GetByID)
		})
		r.Route("/admin", func(r chi.Router) {
			r.Use(authmiddleware.AuthMiddleware(jwtManager))
			r.Use(authmiddleware.RequireRole("admin"))
			r.Get("/users", admin.ListUsers)
			r.Get("/users/{user_id}", admin.GetUser)
			r.Post("/users/{user_id}/roles", admin.AddRole)
			r.Delete("/users/{user_id}/roles/admin", admin.RemoveAdminRole)
		})
	})
	return r
}

func otelHTTPMiddleware(next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, "http.server",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

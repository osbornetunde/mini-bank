package api

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// h is a helper function that converts http.HandlerFunc to http.Handler.
// This eliminates repetitive http.HandlerFunc() conversions in route definitions.
func h(handler http.HandlerFunc) http.Handler {
	return handler
}

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint (public)
	mux.Handle("GET /health", h(a.HealthCheckHandler))

	// Prometheus metrics endpoint (protected with Basic Auth)
	mux.Handle("GET /metrics", BasicAuthMiddleware(a.metricsUsername, a.metricsPassword)(promhttp.Handler()))

	// Account routes
	mux.Handle("POST /api/v1/accounts", a.AuthMiddleware(h(a.CreateAccountHandler)))
	mux.Handle("GET /api/v1/accounts", a.AuthMiddleware(h(a.GetAccountsHandler)))
	mux.Handle("GET /api/v1/accounts/{id}", a.AuthMiddleware(h(a.GetAccountHandler)))
	mux.Handle("PUT /api/v1/accounts/overdraft", a.AuthMiddleware(h(a.UpdateOverdraftLimitHandler)))

	// Transaction routes
	mux.Handle("POST /api/v1/transactions/transfer", a.AuthMiddleware(h(a.TransferHandler)))
	mux.Handle("POST /api/v1/transactions/payment", a.AuthMiddleware(h(a.PaymentHandler)))
	mux.Handle("GET /api/v1/accounts/{id}/transactions", a.AuthMiddleware(h(a.GetTransactionsHandler)))
	mux.Handle("GET /api/v1/transactions/{ref}", a.AuthMiddleware(h(a.GetTransactionHandler)))

	// User routes
	mux.Handle("POST /api/v1/users/create", h(a.CreateUserHandler))
	mux.Handle("GET /api/v1/users", a.AuthMiddleware(h(a.GetUsersHandler)))
	mux.Handle("GET /api/v1/users/{id}", a.AuthMiddleware(h(a.GetUserHandler)))
	mux.Handle("PUT /api/v1/users/{id}", a.AuthMiddleware(h(a.UpdateUserHandler)))
	mux.Handle("DELETE /api/v1/users/{id}", a.AuthMiddleware(h(a.DeleteUserHandler)))

	// Authentication routes
	mux.Handle("POST /api/v1/login", h(a.LoginHandler))
	mux.Handle("POST /api/v1/refresh", h(a.RefreshTokenHandler))

	// Password reset routes
	mux.Handle("POST /api/v1/password-reset/request", a.RateLimitMiddleware(h(a.RequestPasswordResetHandler), 5, time.Hour))
	mux.Handle("POST /api/v1/password-reset/confirm", a.RateLimitMiddleware(h(a.ResetPasswordHandler), 5, time.Hour))

	return mux
}

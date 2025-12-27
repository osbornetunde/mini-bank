package api

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// asHandler is a helper function that converts http.HandlerFunc to http.Handler.
// This eliminates repetitive http.HandlerFunc() conversions in route definitions.
func asHandler(handler http.HandlerFunc) http.Handler {
	return handler
}

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint (public)
	mux.Handle("GET /health", asHandler(a.HealthCheckHandler))

	// Prometheus metrics endpoint (protected with Basic Auth)
	metricsHandler := BasicAuthMiddleware(a.metricsUsername, a.metricsPassword)(promhttp.Handler())
	mux.Handle("GET /metrics", metricsHandler)

	// Account routes
	mux.Handle("POST /api/v1/accounts", a.AuthMiddleware(asHandler(a.CreateAccountHandler)))
	mux.Handle("GET /api/v1/accounts", a.AuthMiddleware(asHandler(a.GetAccountsHandler)))
	mux.Handle("GET /api/v1/accounts/{id}", a.AuthMiddleware(asHandler(a.GetAccountHandler)))
	mux.Handle("PUT /api/v1/accounts/overdraft", a.AuthMiddleware(asHandler(a.UpdateOverdraftLimitHandler)))

	// Transaction routes
	mux.Handle("POST /api/v1/transactions/transfer", a.AuthMiddleware(asHandler(a.TransferHandler)))
	mux.Handle("POST /api/v1/transactions/payment", a.AuthMiddleware(asHandler(a.PaymentHandler)))
	mux.Handle("GET /api/v1/accounts/{id}/transactions", a.AuthMiddleware(asHandler(a.GetTransactionsHandler)))
	mux.Handle("GET /api/v1/transactions/{ref}", a.AuthMiddleware(asHandler(a.GetTransactionHandler)))

	// User routes
	mux.Handle("POST /api/v1/users/create", asHandler(a.CreateUserHandler))
	mux.Handle("GET /api/v1/users", a.AuthMiddleware(asHandler(a.GetUsersHandler)))
	mux.Handle("GET /api/v1/users/{id}", a.AuthMiddleware(asHandler(a.GetUserHandler)))
	mux.Handle("PUT /api/v1/users/{id}", a.AuthMiddleware(asHandler(a.UpdateUserHandler)))
	mux.Handle("DELETE /api/v1/users/{id}", a.AuthMiddleware(asHandler(a.DeleteUserHandler)))

	// Authentication routes
	mux.Handle("POST /api/v1/login", asHandler(a.LoginHandler))
	mux.Handle("POST /api/v1/refresh", asHandler(a.RefreshTokenHandler))

	// Password reset routes
	mux.Handle("POST /api/v1/password-reset/request", a.RateLimitMiddleware(asHandler(a.RequestPasswordResetHandler), 5, time.Hour))
	mux.Handle("POST /api/v1/password-reset/confirm", a.RateLimitMiddleware(asHandler(a.ResetPasswordHandler), 5, time.Hour))

	return mux
}

package api

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint (public)
	mux.HandleFunc("GET /health", a.HealthCheckHandler)

	// Prometheus metrics endpoint (protected with Basic Auth)
	metricsHandler := BasicAuthMiddleware(a.metricsUsername, a.metricsPassword)(promhttp.Handler())
	mux.Handle("GET /metrics", metricsHandler)

	// Account routes
	mux.HandleFunc("POST /api/v1/accounts", a.AuthMiddleware(a.CreateAccountHandler))
	mux.HandleFunc("GET /api/v1/accounts", a.AuthMiddleware(a.GetAccountsHandler))
	mux.HandleFunc("GET /api/v1/accounts/{id}", a.AuthMiddleware(a.GetAccountHandler))
	mux.HandleFunc("PUT /api/v1/accounts/overdraft", a.AuthMiddleware(a.UpdateOverdraftLimitHandler))

	// Transaction routes
	mux.HandleFunc("POST /api/v1/transactions/transfer", a.AuthMiddleware(a.TransferHandler))
	mux.HandleFunc("POST /api/v1/transactions/payment", a.AuthMiddleware(a.PaymentHandler))
	mux.HandleFunc("GET /api/v1/accounts/{id}/transactions", a.AuthMiddleware(a.GetTransactionsHandler))
	mux.HandleFunc("GET /api/v1/transactions/{ref}", a.AuthMiddleware(a.GetTransactionHandler))

	// User routes
	mux.HandleFunc("POST  /api/v1/users/create", a.CreateUserHandler)
	mux.HandleFunc("GET /api/v1/users", a.AuthMiddleware(a.GetUsersHandler))
	mux.HandleFunc("GET /api/v1/users/{id}", a.AuthMiddleware(a.GetUserHandler))
	mux.HandleFunc("PUT /api/v1/users/{id}", a.AuthMiddleware(a.UpdateUserHandler))
	mux.HandleFunc("DELETE /api/v1/users/{id}", a.AuthMiddleware(a.DeleteUserHandler))

	// Authentication routes
	mux.HandleFunc("POST /api/v1/login", a.LoginHandler)
	mux.HandleFunc("POST /api/v1/refresh", a.RefreshTokenHandler)

	// Password reset routes
	mux.HandleFunc("POST /api/v1/password-reset/request", a.RateLimitMiddleware(a.RequestPasswordResetHandler, 5, time.Hour))
	mux.HandleFunc("POST /api/v1/password-reset/confirm", a.RateLimitMiddleware(a.ResetPasswordHandler, 5, time.Hour))

	return mux
}

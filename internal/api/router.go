package api

import (
	"mini-bank/internal/core"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// asHandler is a helper function that converts http.HandlerFunc to http.Handler.
// This eliminates repetitive http.HandlerFunc() conversions in route definitions.
func asHandler(handler http.HandlerFunc) http.Handler {
	return handler
}

// MiddlewareOption is a function that wraps an http.Handler with additional middleware.
type MiddlewareOption func(http.Handler) http.Handler

// withMiddleware applies authentication, permission checking, and any additional middleware options.
// This is the composable approach for applying middleware to routes.
//
// Examples:
//
//	// Auth + permission only
//	mux.Handle("POST /api/v1/accounts", a.withMiddleware(core.PermAccountsWrite, a.CreateAccountHandler))
//
//	// Auth + permission + rate limiting
//	mux.Handle("POST /api/v1/transfer", a.withMiddleware(core.PermTransactionsProcess, a.TransferHandler,
//	    a.withRateLimit(10, time.Minute)))
//
//	// Auth + permission + multiple middleware
//	mux.Handle("POST /api/v1/sensitive", a.withMiddleware(core.PermSomething, a.SomeHandler,
//	    a.withRateLimit(10, time.Minute),
//	    a.withAuditLog("action")))
func (a *API) withMiddleware(permission string, handler http.HandlerFunc, opts ...MiddlewareOption) http.Handler {
	h := asHandler(handler)

	// Apply additional middleware in reverse order (so first option wraps innermost)
	for i := len(opts) - 1; i >= 0; i-- {
		h = opts[i](h)
	}

	// Apply permission check and auth (always applied)
	h = a.RequirePermission(permission)(h)
	h = a.AuthMiddleware(h)

	return h
}

// Middleware option helpers - compose these as needed

func (a *API) withRateLimit(limit int, window time.Duration) MiddlewareOption {
	return func(h http.Handler) http.Handler {
		return a.RateLimitMiddleware(h, limit, window)
	}
}

// Example: Add more middleware options as needed
// func (a *API) withAuditLog(action string) MiddlewareOption {
// 	return func(h http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			userID := r.Context().Value(contextKeyUserID).(int)
// 			a.logger.Info("audit", "action", action, "user_id", userID)
// 			h.ServeHTTP(w, r)
// 		})
// 	}
// }

// func (a *API) withIPWhitelist(allowedIPs []string) MiddlewareOption {
// 	return func(h http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			clientIP := getRealIP(r, a.trustProxy)
// 			for _, ip := range allowedIPs {
// 				if ip == clientIP {
// 					h.ServeHTTP(w, r)
// 					return
// 				}
// 			}
// 			httpError(w, http.StatusForbidden, "IP not whitelisted")
// 		})
// 	}
// }

// func (a *API) withRequestSizeLimit(maxBytes int64) MiddlewareOption {
// 	return func(h http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
// 			h.ServeHTTP(w, r)
// 		})
// 	}
// }

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint (public)
	mux.Handle("GET /health", asHandler(a.HealthCheckHandler))

	// Prometheus metrics endpoint (protected with Basic Auth)
	metricsHandler := BasicAuthMiddleware(a.metricsUsername, a.metricsPassword)(promhttp.Handler())
	mux.Handle("GET /metrics", metricsHandler)

	// Account routes
	mux.Handle("POST /api/v1/accounts", a.withMiddleware(core.PermAccountsWrite, a.CreateAccountHandler))
	mux.Handle("GET /api/v1/accounts", a.withMiddleware(core.PermAccountsRead, a.GetAccountsHandler))
	mux.Handle("GET /api/v1/accounts/{id}", a.withMiddleware(core.PermAccountsRead, a.GetAccountHandler))
	mux.Handle("PUT /api/v1/accounts/overdraft", a.withMiddleware(core.PermAccountsUpdate, a.UpdateOverdraftLimitHandler))

	// Transaction routes
	// Example: Add rate limiting to sensitive financial operations
	// mux.Handle("POST /api/v1/transactions/transfer", a.withMiddleware(core.PermTransactionsProcess, a.TransferHandler, a.withRateLimit(10, time.Minute)))
	mux.Handle("POST /api/v1/transactions/transfer", a.withMiddleware(core.PermTransactionsProcess, a.TransferHandler))
	mux.Handle("POST /api/v1/transactions/payment", a.withMiddleware(core.PermTransactionsProcess, a.PaymentHandler))
	mux.Handle("GET /api/v1/accounts/{id}/transactions", a.withMiddleware(core.PermTransactionsRead, a.GetTransactionsHandler))
	mux.Handle("GET /api/v1/transactions/{ref}", a.withMiddleware(core.PermTransactionsRead, a.GetTransactionHandler))
	mux.Handle("POST /api/v1/transactions/withdraw", a.withMiddleware(core.PermTransactionsProcess, a.WithdrawHandler))

	// User routes
	mux.Handle("POST /api/v1/users/create", asHandler(a.CreateUserHandler))
	mux.Handle("GET /api/v1/users", a.withMiddleware(core.PermUsersRead, a.GetUsersHandler))
	mux.Handle("GET /api/v1/users/{id}", a.withMiddleware(core.PermUsersRead, a.GetUserHandler))
	mux.Handle("PUT /api/v1/users/{id}", a.withMiddleware(core.PermUsersUpdate, a.UpdateUserHandler))
	mux.Handle("DELETE /api/v1/users/{id}", a.withMiddleware(core.PermUsersWrite, a.DeleteUserHandler))

	// Permission management routes
	mux.Handle("GET /api/v1/users/{id}/permissions", a.withMiddleware(core.PermPermissionsManage, a.GetUserPermissionsHandler))
	mux.Handle("PUT /api/v1/users/{id}/permissions", a.withMiddleware(core.PermPermissionsManage, a.UpdateUserPermissionsHandler))

	// Authentication routes
	mux.Handle("POST /api/v1/login", asHandler(a.LoginHandler))
	mux.Handle("POST /api/v1/refresh", asHandler(a.RefreshTokenHandler))

	// Password reset routes
	mux.Handle("POST /api/v1/password-reset/request", a.RateLimitMiddleware(asHandler(a.RequestPasswordResetHandler), 5, time.Hour))
	mux.Handle("POST /api/v1/password-reset/confirm", a.RateLimitMiddleware(asHandler(a.ResetPasswordHandler), 5, time.Hour))

	return mux
}

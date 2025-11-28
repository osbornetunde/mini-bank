package api

import (
	"context"
	"net/http"
	"time"

)

// responseRecorder wraps http.ResponseWriter to capture the status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	// Default to 200 OK if WriteHeader is not called.
	return &responseRecorder{w, http.StatusOK}
}

// WriteHeader captures the status code before calling the original WriteHeader.
func (rr *responseRecorder) WriteHeader(statusCode int) {
	rr.statusCode = statusCode
	rr.ResponseWriter.WriteHeader(statusCode)
}

// LoggingMiddleware logs details about each incoming request.
func (a *API) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rr := newResponseRecorder(w)

		next.ServeHTTP(rr, r)

		duration := time.Since(start)

		a.logger.Info("processed request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", duration,
			"status", rr.statusCode,
			"user_agent", r.UserAgent(),
		)
	})
}

func (a *API) TimeoutMiddleware(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

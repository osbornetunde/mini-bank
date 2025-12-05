package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
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


type contextKey string

const contextKeyUserID contextKey = "user_id"

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

func (a *API) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		tokenString := ""
		if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "Invalid Authorization Header", http.StatusUnauthorized)
				return
			}
		tokenString = authHeader[7:]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(a.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			a.logger.Warn("invalid token", "err", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid token claims", http.StatusUnauthorized)
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, int(userIDFloat))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) AuthenticationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer") {
			http.Error(w, "Invalid Authorization Header", http.StatusUnauthorized)
			return
		}
		tokenString := authHeader[7:]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			return []byte(a.jwtSecret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token or expired token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"mini-bank/internal/service"
	"mini-bank/pkg/metrics"

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

const (
	contextKeyUserID      contextKey = "user_id"
	contextKeyPermissions contextKey = "permissions"
)

// getRealIP extracts the real client IP address from the request.
// It checks proxy headers (X-Forwarded-For, X-Real-IP) if the application
// is configured to trust proxies, otherwise falls back to RemoteAddr.
//
// When behind a reverse proxy (nginx, AWS ALB, Cloudflare, etc.), the
// RemoteAddr will be the proxy's IP, not the client's. This function
// properly extracts the client IP based on standard proxy headers.
//
// Security notes:
// - Only trust proxy headers if trustProxy is true
// - X-Forwarded-For can contain multiple IPs (client, proxy1, proxy2, ...)
// - We take the first (leftmost) IP as the original client
// - Always validate and sanitize the extracted IP
func getRealIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// X-Forwarded-For is the standard header set by proxies
		// Format: "client, proxy1, proxy2"
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Split by comma and take the first (original client) IP
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				clientIP := strings.TrimSpace(ips[0])
				if clientIP != "" {
					return clientIP
				}
			}
		}

		// X-Real-IP is set by some proxies (nginx, Cloudflare)
		// This contains only the client IP, not a chain
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}

		// CF-Connecting-IP is set by Cloudflare
		if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
			return strings.TrimSpace(cfIP)
		}
	}

	// Fallback to RemoteAddr (direct connection or proxy headers not trusted)
	// Use net.SplitHostPort to handle both IPv4 and IPv6 addresses correctly
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, RemoteAddr might not have a port
		// This is unusual but valid - just return it as-is
		return r.RemoteAddr
	}
	return host
}

// LoggingMiddleware logs details about each incoming request.
func (a *API) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ip := getRealIP(r, a.trustProxy)
		ua := r.UserAgent()

		ctx := context.WithValue(r.Context(), service.ContextKeyIP, ip)
		ctx = context.WithValue(ctx, service.ContextKeyUserAgent, ua)
		r = r.WithContext(ctx)

		rr := newResponseRecorder(w)

		next.ServeHTTP(rr, r)

		duration := time.Since(start)

		// Record metrics
		metrics.HttpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rr.statusCode)).Inc()
		metrics.HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

		a.logger.Info("processed request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", duration,
			"status", rr.statusCode,
			"user_agent", ua,
			"ip", ip,
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

func (a *API) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		tokenString := ""
		if !strings.HasPrefix(authHeader, "Bearer ") {
			httpError(w, http.StatusUnauthorized, "Invalid Authorization Header")
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
			httpError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			httpError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			httpError(w, http.StatusUnauthorized, "Invalid user ID in token")
			return
		}

		userID := int(userIDFloat)

		// Extract permissions from JWT claims
		permissions := []string{}
		if permsInterface, ok := claims["permissions"]; ok {
			if permsList, ok := permsInterface.([]interface{}); ok {
				for _, p := range permsList {
					if permStr, ok := p.(string); ok {
						permissions = append(permissions, permStr)
					}
				}
			}
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
		ctx = context.WithValue(ctx, contextKeyPermissions, permissions)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission checks if the authenticated user has the specified permission.
// Must be used after AuthMiddleware.
func (a *API) RequirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			perms, ok := r.Context().Value(contextKeyPermissions).([]string)
			if !ok {
				httpError(w, http.StatusForbidden, "permission denied")
				return
			}

			if !slices.Contains(perms, permission) {
				a.logger.Warn("permission denied",
					"required", permission,
					"user_permissions", perms,
				)
				httpError(w, http.StatusForbidden, "permission denied")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (a *API) RateLimitMiddleware(next http.Handler, limit int, window time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r, a.trustProxy)

		key := fmt.Sprintf("ratelimit:%s:%s", r.URL.Path, ip)

		val, err := a.redis.Incr(r.Context(), key).Result()
		if err != nil {
			a.logger.Error("rate limit error", "err", err)
			// Fail open
			next.ServeHTTP(w, r)
			return
		}

		if val == 1 {
			a.redis.Expire(r.Context(), key, window)
		}

		if val > int64(limit) {
			httpError(w, http.StatusTooManyRequests, "Too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// basicAuthCheck performs the actual basic authentication validation.
// This is shared by both BasicAuthMiddleware and BasicAuthMiddlewareFunc.
func basicAuthCheck(w http.ResponseWriter, r *http.Request, username, password string) bool {
	// If no credentials are configured, deny access
	if username == "" || password == "" {
		http.Error(w, "Metrics authentication not configured", http.StatusServiceUnavailable)
		return false
	}

	// Get credentials from request
	user, pass, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Metrics"`)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

	if !usernameMatch || !passwordMatch {
		w.Header().Set("WWW-Authenticate", `Basic realm="Metrics"`)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return false
	}

	return true
}

// BasicAuthMiddleware provides HTTP Basic Authentication for an endpoint.
// This version wraps http.Handler and is used for third-party handlers like Prometheus.
// For function-based handlers, use BasicAuthMiddlewareFunc instead.
func BasicAuthMiddleware(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !basicAuthCheck(w, r, username, password) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security-related HTTP headers to all responses.
// These headers protect against common web vulnerabilities like clickjacking,
// MIME sniffing, and XSS attacks.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking attacks by restricting who can embed this page in iframes
		// DENY = no one can frame this page
		w.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME-sniffing attacks by forcing browsers to respect Content-Type
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable browser XSS protection (legacy, but still good to have)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Restrict what resources the browser can load (Content Security Policy)
		// This is a restrictive policy suitable for an API
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Enforce HTTPS for future requests (HSTS)
		// max-age=31536000 = 1 year
		// includeSubDomains = apply to all subdomains
		// Note: Only set this if the application is accessed over HTTPS
		// In production behind HTTPS, uncomment this:
		// w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Indicate that the browser should not send referrer information
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Disable features that aren't needed (Permissions Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

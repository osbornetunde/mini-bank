package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockService struct {
	service.Service
	mock.Mock
}

func (m *mockService) GetUser(ctx context.Context, id int) (*core.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.User), args.Error(1)
}

func TestRequirePermission(t *testing.T) {
	api := &API{
		logger: slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)),
	}

	tests := []struct {
		name           string
		userPerms      []string
		requiredPerm   string
		expectedStatus int
	}{
		{
			name:           "Has permission",
			userPerms:      []string{"accounts_read", "accounts_write"},
			requiredPerm:   "accounts_read",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing permission",
			userPerms:      []string{"accounts_write"},
			requiredPerm:   "accounts_read",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "No permissions",
			userPerms:      []string{},
			requiredPerm:   "accounts_read",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := api.RequirePermission(tt.requiredPerm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			ctx := context.WithValue(req.Context(), contextKeyPermissions, tt.userPerms)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	mockSvc := new(mockService)
	jwtSecret := "test-secret"
	api := &API{
		service:   mockSvc,
		logger:    slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)),
		jwtSecret: jwtSecret,
	}

	userID := 123
	permissions := []string{"accounts_read", "transactions_write"}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":     float64(userID),
		"permissions": permissions,
		"exp":         time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(jwtSecret))

	handler := api.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxPerms, ok := r.Context().Value(contextKeyPermissions).([]string)
		assert.True(t, ok)
		assert.Equal(t, permissions, ctxPerms)

		ctxUserID, ok := r.Context().Value(contextKeyUserID).(int)
		assert.True(t, ok)
		assert.Equal(t, userID, ctxUserID)

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockSvc.AssertExpectations(t)
}

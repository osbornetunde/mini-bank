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
	"mini-bank/internal/storage"

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

func (m *mockService) GetUsers(ctx context.Context, pagination storage.PaginationParams) (*storage.UsersPaginatedResult, error) {
	args := m.Called(ctx, pagination)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.UsersPaginatedResult), args.Error(1)
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

func TestGetRealIP(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		trustProxy  bool
		headers     map[string]string
		expectedIP  string
		description string
	}{
		{
			name:        "IPv4 localhost",
			remoteAddr:  "127.0.0.1:12345",
			trustProxy:  false,
			expectedIP:  "127.0.0.1",
			description: "Should strip port from IPv4",
		},
		{
			name:        "IPv6 localhost with brackets",
			remoteAddr:  "[::1]:12345",
			trustProxy:  false,
			expectedIP:  "::1",
			description: "Should strip brackets and port from IPv6",
		},
		{
			name:        "IPv6 full address",
			remoteAddr:  "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:8080",
			trustProxy:  false,
			expectedIP:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			description: "Should handle full IPv6 address",
		},
		{
			name:        "IPv4 from network",
			remoteAddr:  "192.168.1.100:54321",
			trustProxy:  false,
			expectedIP:  "192.168.1.100",
			description: "Should strip port from network IPv4",
		},
		{
			name:       "X-Forwarded-For trusted",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1",
			},
			expectedIP:  "203.0.113.1",
			description: "Should use first IP from X-Forwarded-For when trusted",
		},
		{
			name:       "X-Forwarded-For not trusted",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: false,
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1, 10.0.0.1",
			},
			expectedIP:  "10.0.0.1",
			description: "Should ignore X-Forwarded-For when not trusted",
		},
		{
			name:       "X-Real-IP trusted",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Real-IP": "203.0.113.2",
			},
			expectedIP:  "203.0.113.2",
			description: "Should use X-Real-IP when trusted",
		},
		{
			name:       "CF-Connecting-IP trusted",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.3",
			},
			expectedIP:  "203.0.113.3",
			description: "Should use CF-Connecting-IP when trusted",
		},
		{
			name:       "X-Forwarded-For with spaces",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": " 203.0.113.4 , 10.0.0.1 ",
			},
			expectedIP:  "203.0.113.4",
			description: "Should trim spaces from X-Forwarded-For",
		},
		{
			name:       "X-Forwarded-For empty",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "",
			},
			expectedIP:  "10.0.0.1",
			description: "Should fallback to RemoteAddr when X-Forwarded-For is empty",
		},
		{
			name:       "X-Forwarded-For only spaces",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "   ",
			},
			expectedIP:  "10.0.0.1",
			description: "Should fallback to RemoteAddr when X-Forwarded-For contains only spaces",
		},
		{
			name:       "Multiple headers - X-Forwarded-For takes precedence",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For":  "203.0.113.1",
				"X-Real-IP":        "203.0.113.2",
				"CF-Connecting-IP": "203.0.113.3",
			},
			expectedIP:  "203.0.113.1",
			description: "X-Forwarded-For should take precedence over other headers",
		},
		{
			name:       "X-Real-IP fallback when XFF empty",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "",
				"X-Real-IP":       "203.0.113.2",
			},
			expectedIP:  "203.0.113.2",
			description: "Should use X-Real-IP when X-Forwarded-For is empty",
		},
		{
			name:       "CF-Connecting-IP fallback",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For":  "",
				"X-Real-IP":        "",
				"CF-Connecting-IP": "203.0.113.3",
			},
			expectedIP:  "203.0.113.3",
			description: "Should use CF-Connecting-IP when X-Forwarded-For and X-Real-IP are empty",
		},
		{
			name:        "IPv6 with zone identifier",
			remoteAddr:  "[fe80::1%eth0]:8080",
			trustProxy:  false,
			expectedIP:  "fe80::1%eth0",
			description: "Should handle IPv6 with zone identifier",
		},
		{
			name:        "RemoteAddr without port",
			remoteAddr:  "192.168.1.1",
			trustProxy:  false,
			expectedIP:  "192.168.1.1",
			description: "Should handle RemoteAddr without port gracefully",
		},
		{
			name:        "IPv6 RemoteAddr without port",
			remoteAddr:  "2001:db8::1",
			trustProxy:  false,
			expectedIP:  "2001:db8::1",
			description: "Should handle IPv6 RemoteAddr without port gracefully",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.5",
			},
			expectedIP:  "203.0.113.5",
			description: "Should handle single IP in X-Forwarded-For",
		},
		{
			name:       "X-Forwarded-For trailing comma",
			remoteAddr: "10.0.0.1:12345",
			trustProxy: true,
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.6,",
			},
			expectedIP:  "203.0.113.6",
			description: "Should handle trailing comma in X-Forwarded-For",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			// Add headers if provided
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := getRealIP(req, tt.trustProxy)
			assert.Equal(t, tt.expectedIP, result, tt.description)
		})
	}
}

func TestToAccountResponse(t *testing.T) {
	t.Run("nil account returns nil", func(t *testing.T) {
		result := toAccountResponse(nil)
		assert.Nil(t, result)
	})

	t.Run("valid account returns response", func(t *testing.T) {
		now := time.Now()
		acc := &core.Account{
			ID:             42,
			UserID:         1,
			Balance:        1500,
			OverdraftLimit: 500,
			CreatedAt:      now,
		}

		result := toAccountResponse(acc)

		assert.NotNil(t, result)
		assert.Equal(t, 42, result.ID)
		assert.Equal(t, 1, result.UserID)
		assert.Equal(t, int64(1500), result.Balance)
		assert.Equal(t, int64(500), result.OverdraftLimit)
		assert.Equal(t, now, result.CreatedAt)
	})

	t.Run("negative balance preserved", func(t *testing.T) {
		acc := &core.Account{
			ID:             1,
			UserID:         1,
			Balance:        -200, // Overdraft
			OverdraftLimit: 500,
			CreatedAt:      time.Now(),
		}

		result := toAccountResponse(acc)

		assert.Equal(t, int64(-200), result.Balance)
	})
}

func TestGetUsersHandler_ResponseStructure(t *testing.T) {
	mockSvc := new(mockService)
	api := &API{
		service: mockSvc,
		logger:  slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)),
	}

	// now := time.Now()
	zeroBalance := int64(0)
	// totalBalance unused in modified tests


	tests := []struct {
		name          string
		mockUsers     []*core.User
		checkResponse func(t *testing.T, body string)
	}{
		{
			name: "User with zero accounts",
			mockUsers: []*core.User{
				{
					ID:          1,
					FirstName:   "John",
					LastName:    "Doe",
					Email:       "john@example.com",
					Balance:     &zeroBalance,
					Permissions: []string{},
					Accounts:    []*core.Account{},
				},
			},
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `"first_name":"John"`)
				assert.Contains(t, body, `"email":"john@example.com"`)
				assert.Contains(t, body, `"balance":0`)
				// Should contain pagination metadata
				assert.Contains(t, body, `"total_items":1`)
				assert.Contains(t, body, `"has_next"`)
				assert.Contains(t, body, `"has_previous"`)
			},
		},
		{
			name: "User with multiple accounts",
			mockUsers: []*core.User{
				{
					ID:          1,
					FirstName:   "Jane",
					LastName:    "Smith",
					Email:       "jane@example.com",
					Balance:     nil, // Balance calculated on fly
					Permissions: []string{"accounts_read"},
					Accounts: []*core.Account{
						{ID: 1, UserID: 1, Balance: 1000},
						{ID: 2, UserID: 1, Balance: 1500},
					},
				},
			},
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, body, `"first_name":"Jane"`)
				assert.Contains(t, body, `"balance":2500`)
				// Should NOT contain accounts list in the main response
				assert.NotContains(t, body, `"accounts":[`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc.ExpectedCalls = nil // Reset mock
			mockSvc.On("GetUsers", mock.Anything, mock.Anything).Return(&storage.UsersPaginatedResult{Users: tt.mockUsers, TotalCount: 1}, nil)

			req := httptest.NewRequest("GET", "/api/v1/users?page=1&limit=10", nil)
			rr := httptest.NewRecorder()

			api.GetUsersHandler(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			body := rr.Body.String()
			tt.checkResponse(t, body)

			mockSvc.AssertExpectations(t)
		})
	}
}

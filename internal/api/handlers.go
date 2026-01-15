package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mini-bank/internal/core"
	"mini-bank/internal/service"
	"mini-bank/internal/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"mini-bank/pkg/metrics"
)

const (
	// Account lockout configuration
	maxFailedLoginAttempts = 5
	accountLockoutDuration = 15 * time.Minute
)

type API struct {
	service         service.Service
	logger          *slog.Logger
	redis           *redis.Client
	jwtSecret       string
	metricsUsername string
	metricsPassword string
	trustProxy      bool
}

func NewAPI(s service.Service, logger *slog.Logger, rdb *redis.Client, jwtSecret, metricsUser, metricsPass string, trustProxy bool) *API {
	return &API{
		service:         s,
		logger:          logger,
		redis:           rdb,
		jwtSecret:       jwtSecret,
		metricsUsername: metricsUser,
		metricsPassword: metricsPass,
		trustProxy:      trustProxy,
	}
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// checkAccountLockout checks if an account is locked due to failed login attempts
func (a *API) checkAccountLockout(ctx context.Context, email string) (bool, int, error) {
	key := fmt.Sprintf("login_attempts:%s", email)

	// Get current failed attempts count
	attempts, err := a.redis.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return false, 0, err
	}

	// Check if account should be locked
	if attempts >= maxFailedLoginAttempts {
		// Get TTL to see how long the lockout lasts
		ttl, err := a.redis.TTL(ctx, key).Result()
		if err != nil {
			return false, 0, err
		}

		// If TTL exists, account is still locked
		if ttl > 0 {
			return true, attempts, nil
		}
	}

	return false, attempts, nil
}

// recordFailedLoginAttempt increments the failed login counter for an account
func (a *API) recordFailedLoginAttempt(ctx context.Context, email string) error {
	key := fmt.Sprintf("login_attempts:%s", email)

	// Increment counter
	attempts, err := a.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Set expiry on first failed attempt
	if attempts == 1 {
		a.redis.Expire(ctx, key, accountLockoutDuration)
	}

	// If we just hit the lockout threshold, increment lockout metric
	if attempts == maxFailedLoginAttempts {
		metrics.AccountLockouts.Inc()
		a.logger.Warn("account locked due to failed login attempts",
			"email", email,
			"attempts", attempts,
			"lockout_duration", accountLockoutDuration,
		)
	}

	return nil
}

// resetFailedLoginAttempts clears the failed login counter after successful login
func (a *API) resetFailedLoginAttempts(ctx context.Context, email string) error {
	key := fmt.Sprintf("login_attempts:%s", email)
	return a.redis.Del(ctx, key).Err()
}

type createAccountRequest struct {
	UserID         int   `json:"user_id"`
	InitialBalance int64 `json:"initial_balance"`
}

type createAccountResponse struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Balance        int64     `json:"balance"`
	OverdraftLimit int64     `json:"overdraft_limit"`
	CreatedAt      time.Time `json:"created_at"`
}

type getAccountResponse struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Balance        int64     `json:"balance"`
	OverdraftLimit int64     `json:"overdraft_limit"`
	CreatedAt      time.Time `json:"created_at"`
}

// toAccountResponse converts a core.Account to an API response.
// This helper reduces duplication across handlers.
func toAccountResponse(acc *core.Account) *getAccountResponse {
	if acc == nil {
		return nil
	}
	return &getAccountResponse{
		ID:             acc.ID,
		UserID:         acc.UserID,
		Balance:        acc.Balance,
		OverdraftLimit: acc.OverdraftLimit,
		CreatedAt:      acc.CreatedAt,
	}
}

type getAccountsResponse struct {
	Accounts []*getAccountResponse `json:"accounts"`
}

type transferResponse struct {
	FromAccount *getAccountResponse `json:"from_account"`
	ToAccount   *getAccountResponse `json:"to_account"`
	Reference   string              `json:"reference,omitempty"`
	Fee         int64               `json:"fee"`
}

type transferRequest struct {
	FromID int   `json:"from_id"`
	ToID   int   `json:"to_id"`
	Amount int64 `json:"amount"`
}

type paymentRequest struct {
	AccountID int                 `json:"account_id"`
	Amount    int64               `json:"amount"`
	Type      storage.PaymentType `json:"type"`
}

type updateOverdraftLimitRequest struct {
	AccountID      int   `json:"account_id"`
	OverdraftLimit int64 `json:"overdraft_limit"`
}

type createUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type createUserResponse struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

type usersResponse struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Balance   int64  `json:"balance"`
}

type userResponse struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Balance   *int64 `json:"balance,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponse struct {
	Token string `json:"token"`
}

type RequestPasswordResetRequest struct {
	Email string `json:"email"`
}

type RequestPasswordResetResponse struct {
	Message string `json:"message"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordResponse struct {
	Message string `json:"message"`
}

type WithdrawRequest struct {
	AccountID *int   `json:"account_id,omitempty"` // Optional: if not provided, uses first account
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
}

type WithdrawResponse struct {
	Balance   int64  `json:"balance"`
	Reference string `json:"reference"`
	Fee       int64  `json:"fee"`
}

func (a *API) CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := req.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	acc, err := a.service.CreateAccount(ctx, req.UserID, req.InitialBalance)
	if err != nil {
		a.logger.Error("failed to create account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to create account")

		return
	}

	resp := createAccountResponse{
		ID:             acc.ID,
		UserID:         acc.UserID,
		Balance:        acc.Balance,
		OverdraftLimit: acc.OverdraftLimit,
		CreatedAt:      acc.CreatedAt,
	}

	jsonResponse(w, http.StatusCreated, resp)
}

func httpError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func (a *API) getAuthorizedAccount(w http.ResponseWriter, r *http.Request, accountID int) *core.Account {
	ctx := r.Context()

	userID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return nil
	}

	acc, err := a.service.GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			httpError(w, http.StatusNotFound, "account not found")
			return nil
		}
		a.logger.Error("failed to get account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve account")
		return nil
	}

	if acc.UserID != userID {
		httpError(w, http.StatusForbidden, "forbidden")
		return nil
	}

	return acc
}

func (a *API) GetAccountHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	acc := a.getAuthorizedAccount(w, r, id)
	if acc == nil {
		return
	}

	jsonResponse(w, http.StatusOK, toAccountResponse(acc))
}

func (a *API) GetAccountsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accounts, err := a.service.ListAccounts(ctx)
	if err != nil {
		a.logger.Error("failed to get accounts", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve accounts")
		return
	}

	var accountsResponse []*getAccountResponse
	for _, acc := range accounts {
		accountsResponse = append(accountsResponse, toAccountResponse(acc))
	}

	jsonResponse(w, http.StatusOK, getAccountsResponse{Accounts: accountsResponse})
}

func (a *API) UpdateOverdraftLimitHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req updateOverdraftLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := req.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Verify account belongs to the authenticated user
	account, err := a.service.GetAccount(ctx, req.AccountID)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			httpError(w, http.StatusNotFound, "account not found")
			return
		}
		a.logger.Error("failed to get account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to process request")
		return
	}

	if account.UserID != userID {
		httpError(w, http.StatusForbidden, "you can only update overdraft limits for your own accounts")
		return
	}

	// Update the overdraft limit
	updatedAccount, err := a.service.UpdateOverdraftLimit(ctx, req.AccountID, req.OverdraftLimit)
	if err != nil {
		a.logger.Error("failed to update overdraft limit", "account_id", req.AccountID, "err", err)
		httpError(w, http.StatusInternalServerError, "failed to update overdraft limit")
		return
	}

	jsonResponse(w, http.StatusOK, toAccountResponse(updatedAccount))
}

func (a *API) TransferHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := req.Validate(); err != nil {
		// Track rejected transactions for monitoring
		if errors.Is(err, ErrAmountTooLarge) {
			metrics.RejectedTransactions.WithLabelValues("amount_exceeds_limit", "transfer").Inc()
		}
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	fromAccount, err := a.service.GetAccount(ctx, req.FromID)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			httpError(w, http.StatusNotFound, "sender account not found")
			return
		}
		a.logger.Error("failed to get sender account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to process transfer")
		return
	}

	if fromAccount.UserID != userID {
		httpError(w, http.StatusForbidden, "you can only transfer from your own accounts")
		return
	}

	reference := uuid.NewString()

	result, err := a.service.Transfer(ctx, req.FromID, req.ToID, req.Amount, reference)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrAccountNotFound):
			httpError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, storage.ErrInsufficientFunds):
			httpError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			a.logger.Error("transfer failed", "err", err)
			httpError(w, http.StatusInternalServerError, "transfer failed")
		}
		return
	}

	resp := transferResponse{
		FromAccount: toAccountResponse(result.FromAccount),
		ToAccount:   toAccountResponse(result.ToAccount),
		Reference:   result.Reference,
		Fee:         result.FeeAmount,
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (a *API) PaymentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req paymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if err := req.Validate(); err != nil {
		// Track rejected transactions for monitoring
		if errors.Is(err, ErrAmountTooLarge) {
			metrics.RejectedTransactions.WithLabelValues("amount_exceeds_limit", string(req.Type)).Inc()
		}
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	acc := a.getAuthorizedAccount(w, r, req.AccountID)
	if acc == nil {
		return
	}

	reference := uuid.NewString()

	paymentResp, err := a.service.Payment(ctx, req.AccountID, req.Amount, req.Type, reference)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrAccountNotFound):
			httpError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, storage.ErrInsufficientFunds):
			httpError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			a.logger.Error("payment failed", "type", req.Type, "err", err)
			errorMessage := fmt.Sprintf("%s failed", req.Type)
			httpError(w, http.StatusInternalServerError, errorMessage)
		}
		return
	}

	jsonResponse(w, http.StatusOK, toAccountResponse(paymentResp))
}

func (a *API) GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	accountID, err := strconv.Atoi(idStr)
	if err != nil || accountID <= 0 {
		httpError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	acc := a.getAuthorizedAccount(w, r, accountID)
	if acc == nil {
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20 // default
	}

	// Parse filters
	filters := storage.TransactionFilters{
		Status:    r.URL.Query().Get("status"),
		Reference: r.URL.Query().Get("reference"),
	}

	// Parse date filters
	if dateFromStr := r.URL.Query().Get("date_from"); dateFromStr != "" {
		dateFrom, err := time.Parse("2006-01-02", dateFromStr)
		if err != nil {
			httpError(w, http.StatusBadRequest, "Invalid date_from format. Use YYYY-MM-DD")
			return
		}
		filters.DateFrom = &dateFrom
	}

	if dateToStr := r.URL.Query().Get("date_to"); dateToStr != "" {
		dateTo, err := time.Parse("2006-01-02", dateToStr)
		if err != nil {
			httpError(w, http.StatusBadRequest, "Invalid date_to format. Use YYYY-MM-DD")
			return
		}
		// Set time to end of day
		dateTo = dateTo.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		filters.DateTo = &dateTo
	}

	// Validate status filter if provided
	if filters.Status != "" && filters.Status != "deposit" && filters.Status != "withdraw" && filters.Status != "transfer" {
		httpError(w, http.StatusBadRequest, "Invalid status. Must be: deposit, withdraw, or transfer")
		return
	}

	// Validate date range
	if filters.DateFrom != nil && filters.DateTo != nil && filters.DateFrom.After(*filters.DateTo) {
		httpError(w, http.StatusBadRequest, "date_from cannot be after date_to")
		return
	}

	// Call paginated service method
	result, err := a.service.ListTransactionsPaginated(ctx, accountID, filters, storage.PaginationParams{
		Limit:  limit,
		Offset: (page - 1) * limit,
	})
	if err != nil {
		a.logger.Error("failed to list transactions", "err", err)
		httpError(w, http.StatusInternalServerError, "could not retrieve transactions")
		return
	}

	// Build pagination metadata
	totalPages := int(math.Ceil(float64(result.TotalCount) / float64(limit)))
	response := map[string]interface{}{
		"data": result.Transactions,
		"pagination": map[string]interface{}{
			"page":         page,
			"limit":        limit,
			"total_items":  result.TotalCount,
			"total_pages":  totalPages,
			"has_next":     page < totalPages,
			"has_previous": page > 1,
		},
	}

	jsonResponse(w, http.StatusOK, response)
}

func (a *API) GetTransactionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("ref")

	if idStr == "" {
		httpError(w, http.StatusBadRequest, "Invalid transaction reference")
		return
	}
	resp, err := a.service.GetTransaction(ctx, idStr)
	if err != nil {
		if errors.Is(err, storage.ErrTransactionNotFound) {
			httpError(w, http.StatusNotFound, "transaction not found")
			return
		}

		a.logger.Error("failed to get transaction", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve transaction")
		return
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (a *API) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var user createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		httpError(w, http.StatusBadRequest, "Invalid user data")
		return
	}
	if err := user.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := a.service.CreateUser(ctx, user.FirstName, user.LastName, user.Email, user.Password)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrDuplicateEmail):
			jsonResponse(w, http.StatusConflict, map[string]string{
				"error": "A user with this email already exists",
			})
		default:
			a.logger.Error("failed to create user", "err", err)
			jsonResponse(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to create user",
			})
		}
		return
	}

	tokenString, err := a.generateJWTToken(resp.ID, resp.Permissions)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate JWT token")
		return
	}
	userResponse := createUserResponse{
		ID:        resp.ID,
		FirstName: resp.FirstName,
		LastName:  resp.LastName,
		Email:     resp.Email,
		Token:     tokenString,
	}
	jsonResponse(w, http.StatusOK, userResponse)
}

func (a *API) GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	pagination := storage.PaginationParams{
		Limit:  limit,
		Offset: (page - 1) * limit,
	}

	result, err := a.service.GetUsers(ctx, pagination)
	if err != nil {
		a.logger.Error("failed to get users", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve users")
		return
	}

	// Initialize with empty slice to ensure JSON returns [] instead of null
	users := make([]*usersResponse, 0, len(result.Users))
	for _, user := range result.Users {
		users = append(users, &usersResponse{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Balance:   user.CalculateTotalBalance(),
		})
	}

	totalPages := int(math.Ceil(float64(result.TotalCount) / float64(limit)))

	response := map[string]interface{}{
		"data": users,
		"pagination": map[string]interface{}{
			"page":         page,
			"limit":        limit,
			"total_items":  result.TotalCount,
			"total_pages":  totalPages,
			"has_next":     page < totalPages,
			"has_previous": page > 1,
		},
	}

	jsonResponse(w, http.StatusOK, response)
}

func (a *API) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userId := r.PathValue("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	authUserID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if id != authUserID {
		httpError(w, http.StatusForbidden, "forbidden")
		return
	}

	user, err := a.service.GetUser(ctx, id)
	if err != nil {
		a.logger.Error("failed to get user", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve user")
		return
	}

	var balance *int64
	if user.Balance != nil {
		b := int64(*user.Balance)
		balance = &b
	}
	response := &userResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Balance:   balance,
	}
	jsonResponse(w, http.StatusOK, response)
}

func (a *API) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var updateData struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	}

	userId := r.PathValue("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	authUserID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if id != authUserID {
		httpError(w, http.StatusForbidden, "forbidden")
		return
	}

	user, err := a.service.UpdateUser(ctx, id, updateData.FirstName, updateData.LastName, updateData.Email)
	if err != nil {
		a.logger.Error("failed to update user", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	var balance *int64
	if user.Balance != nil {
		b := int64(*user.Balance)
		balance = &b
	}
	response := &userResponse{
		ID:        user.ID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Balance:   balance,
	}
	jsonResponse(w, http.StatusOK, response)
}

func (a *API) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userId := r.PathValue("id")
	id, err := strconv.Atoi(userId)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	authUserID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if id != authUserID {
		httpError(w, http.StatusForbidden, "forbidden")
		return
	}

	err = a.service.DeleteUser(ctx, id)
	if err != nil {
		a.logger.Error("failed to delete user", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "user deleted successfully"})
}

func (a *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var request LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if request.Email == "" || request.Password == "" {
		httpError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Check if account is locked
	locked, attempts, err := a.checkAccountLockout(ctx, request.Email)
	if err != nil {
		a.logger.Error("failed to check account lockout", "err", err)
		// Continue with login attempt - don't fail due to Redis issues
	}

	if locked {
		a.logger.Warn("login attempt on locked account",
			"email", request.Email,
			"attempts", attempts,
		)
		httpError(w, http.StatusTooManyRequests,
			"Account temporarily locked due to multiple failed login attempts. Please try again later.")
		return
	}

	data, err := a.service.Login(ctx, request.Email, request.Password)
	if err != nil {
		// Record failed attempt
		if recErr := a.recordFailedLoginAttempt(ctx, request.Email); recErr != nil {
			a.logger.Error("failed to record login attempt", "err", recErr)
		}

		// Increment failed login metric
		metrics.FailedLoginAttempts.Inc()

		// We log the actual error for debugging but return a generic message to the user
		a.logger.Warn("login failed", "email", request.Email, "err", err)
		httpError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Successful login - reset failed attempts
	if resetErr := a.resetFailedLoginAttempts(ctx, request.Email); resetErr != nil {
		a.logger.Error("failed to reset failed login attempts", "err", resetErr)
		// Don't fail the login just because we couldn't reset the counter
	}

	token, err := a.generateJWTToken(data.ID, data.Permissions)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	refreshToken, err := a.generateRefreshToken(ctx, data.ID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"token": token, "refresh_token": refreshToken})
}

func (a *API) RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract and validate the expired JWT from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		httpError(w, http.StatusUnauthorized, "authorization header required")
		return
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		httpError(w, http.StatusUnauthorized, "invalid authorization header")
		return
	}

	tokenString := authHeader[7:]

	// Parse JWT without validating expiration (but still validate signature)
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, err := parser.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.jwtSecret), nil
	})

	if err != nil {
		a.logger.Warn("failed to parse JWT for refresh", "err", err)
		httpError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		httpError(w, http.StatusUnauthorized, "invalid token claims")
		return
	}

	jwtUserID, ok := claims["user_id"].(float64)
	if !ok {
		httpError(w, http.StatusUnauthorized, "invalid user_id in token")
		return
	}

	var request RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	key := fmt.Sprintf("session:%s", request.RefreshToken)
	userIDstr, err := a.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		httpError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	} else if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to get token")
		return
	}

	userID, err := strconv.Atoi(userIDstr)
	if err != nil {
		a.logger.Error("invalid user ID in refresh token", "user_id", userIDstr, "err", err)
		httpError(w, http.StatusInternalServerError, "invalid session data")
		return
	}

	// Verify that the JWT user ID matches the refresh token's user ID
	if int(jwtUserID) != userID {
		a.logger.Warn("refresh token user ID mismatch", "jwt_user_id", int(jwtUserID), "refresh_user_id", userID)
		httpError(w, http.StatusUnauthorized, "token mismatch")
		return
	}

	if err := a.redis.Del(ctx, key).Err(); err != nil {
		a.logger.Error("failed to delete old refresh token", "err", err)
	}

	// Fetch user to get current permissions
	user, err := a.service.GetUser(ctx, userID)
	if err != nil {
		a.logger.Error("failed to fetch user for token refresh", "user_id", userID, "err", err)
		httpError(w, http.StatusUnauthorized, "user not found")
		return
	}

	newToken, err := a.generateJWTToken(userID, user.Permissions)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	newRefreshToken, err := a.generateRefreshToken(ctx, userID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"token": newToken, "refresh_token": newRefreshToken})
}

func (a *API) generateJWTToken(userID int, permissions []string) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"user_id":     userID,
			"permissions": permissions,
			"exp":         time.Now().Add(time.Minute * 10).Unix(),
			"app":         "mini-bank",
		},
	)
	tokenString, err := token.SignedString([]byte(a.jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (a *API) generateRefreshToken(ctx context.Context, userID int) (string, error) {
	token := uuid.New().String()

	key := fmt.Sprintf("session:%s", token)

	err := a.redis.Set(ctx, key, userID, time.Hour*24*7).Err()
	if err != nil {
		return "", err
	}

	setKey := fmt.Sprintf("user_sessions:%d", userID)
	err = a.redis.SAdd(ctx, setKey, token).Err()
	if err != nil {
		// If we fail to track it, we should probably delete the session to be safe,
		// but for now we'll just log and continue or return error.
		// Returning error is safer to ensure consistency.
		a.redis.Del(ctx, key)
		return "", err
	}
	a.redis.Expire(ctx, setKey, time.Hour*24*7)

	return token, nil
}

func (a *API) RequestPasswordResetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var request RequestPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := request.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Don't log or return the token to the user
	if _, err := a.service.RequestPasswordReset(ctx, request.Email); err != nil {
		a.logger.Error("failed to create password reset token", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to process password reset request")
		return
	}

	// Always return success to prevent user enumeration
	jsonResponse(w, http.StatusOK, RequestPasswordResetResponse{
		Message: "If the email exists, a password reset link will be sent",
	})
}

func (a *API) ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var request ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := request.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := a.service.ResetPassword(ctx, request.Token, request.NewPassword)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidResetToken) {
			httpError(w, http.StatusBadRequest, "invalid or expired reset token")
			return
		}
		a.logger.Error("failed to reset password", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	if err := a.invalidateUserSessions(ctx, user.ID); err != nil {
		// Log error but don't fail the request as password is already reset
		a.logger.Error("failed to invalidate sessions after password reset", "user_id", user.ID, "err", err)
	}

	jsonResponse(w, http.StatusOK, ResetPasswordResponse{
		Message: "password reset successfully",
	})
}

func (a *API) invalidateUserSessions(ctx context.Context, userID int) error {
	// Get all sessions for the user
	setKey := fmt.Sprintf("user_sessions:%d", userID)
	tokens, err := a.redis.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		return nil
	}

	pipeline := a.redis.Pipeline()
	for _, token := range tokens {
		pipeline.Del(ctx, fmt.Sprintf("session:%s", token))
	}
	pipeline.Del(ctx, setKey)

	_, err = pipeline.Exec(ctx)
	return err
}

func (a *API) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "up"})
}

func (a *API) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Reference == "" {
		httpError(w, http.StatusBadRequest, "reference is required")
		return
	}

	if req.Amount <= 0 {
		httpError(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}

	var accountID int

	// Check if specific account was provided
	if req.AccountID != nil {
		// User specified an account - verify ownership
		acc := a.getAuthorizedAccount(w, r, *req.AccountID)
		if acc == nil {
			return
		}
		accountID = acc.ID
	} else {
		// No account specified - use user's first/primary account
		userID, ok := ctx.Value(contextKeyUserID).(int)
		if !ok {
			httpError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		accounts, err := a.service.ListUserAccounts(ctx, userID)
		if err != nil {
			a.logger.Error("failed to get user accounts", "err", err)
			httpError(w, http.StatusInternalServerError, "failed to process withdrawal")
			return
		}

		if len(accounts) == 0 {
			httpError(w, http.StatusNotFound, "no account found for user")
			return
		}

		// Use the first account as default
		accountID = accounts[0].ID
	}

	// Check if transaction with this reference already exists (idempotency)
	existingTx, err := a.service.GetTransaction(ctx, req.Reference)
	if err == nil {
		// Transaction already exists - return current balance with stored fee (idempotent behavior)
		account, err := a.service.GetAccount(ctx, accountID)
		if err != nil {
			a.logger.Error("failed to get account for existing transaction", "err", err)
			httpError(w, http.StatusInternalServerError, "failed to process withdrawal")
			return
		}
		res := WithdrawResponse{
			Balance:   account.Balance,
			Reference: existingTx.Reference,
			Fee:       existingTx.FeeAmount, // Use stored fee from original transaction
		}
		jsonResponse(w, http.StatusOK, res)
		return
	}

	// Transaction doesn't exist yet (or other error) - process withdrawal
	if !errors.Is(err, storage.ErrTransactionNotFound) {
		a.logger.Error("failed to check existing transaction", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to process withdrawal")
		return
	}

	result, err := a.service.Withdraw(ctx, accountID, req.Amount, req.Reference)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrAccountNotFound):
			httpError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, storage.ErrInsufficientFunds):
			httpError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			a.logger.Error("withdrawal failed", "err", err)
			httpError(w, http.StatusInternalServerError, "withdrawal failed")
		}
		return
	}
	res := WithdrawResponse{
		Balance:   result.Account.Balance,
		Reference: result.Reference,
		Fee:       result.FeeAmount,
	}
	jsonResponse(w, http.StatusOK, res)
}

type UpdatePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type PermissionsResponse struct {
	UserID      int      `json:"user_id"`
	Permissions []string `json:"permissions"`
}

// UpdateUserPermissionsHandler updates the permissions for a user.
// Only users with the permissions_manage permission can access this endpoint.
func (a *API) UpdateUserPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr := r.PathValue("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req UpdatePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate permissions
	if req.Permissions == nil {
		httpError(w, http.StatusBadRequest, "permissions field is required")
		return
	}

	if err := a.service.UpdateUserPermissions(ctx, userID, req.Permissions); err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			httpError(w, http.StatusNotFound, "user not found")
			return
		}
		a.logger.Error("failed to update permissions", "err", err)
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, PermissionsResponse{
		UserID:      userID,
		Permissions: req.Permissions,
	})
}

// GetUserPermissionsHandler returns the permissions for a user.
func (a *API) GetUserPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userIDStr := r.PathValue("id")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	user, err := a.service.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			httpError(w, http.StatusNotFound, "user not found")
			return
		}
		a.logger.Error("failed to get user", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	jsonResponse(w, http.StatusOK, PermissionsResponse{
		UserID:      userID,
		Permissions: user.Permissions,
	})
}

// Fee management handlers

type CreateFeeTierRequest struct {
	TransactionType string   `json:"transaction_type"`
	MinAmount       int64    `json:"min_amount"`
	MaxAmount       *int64   `json:"max_amount"`
	FeeType         string   `json:"fee_type"`
	FlatFee         *int64   `json:"flat_fee"`
	PercentageFee   *float64 `json:"percentage_fee"`
}

func (r *CreateFeeTierRequest) Validate() error {
	if r.TransactionType != "transfer" && r.TransactionType != "withdraw" {
		return errors.New("transaction_type must be 'transfer' or 'withdraw'")
	}
	if r.MinAmount < 0 {
		return errors.New("min_amount must be non-negative")
	}
	if r.MaxAmount != nil && *r.MaxAmount <= r.MinAmount {
		return errors.New("max_amount must be greater than min_amount")
	}
	if r.FeeType != "flat" && r.FeeType != "percentage" && r.FeeType != "combined" {
		return errors.New("fee_type must be 'flat', 'percentage', or 'combined'")
	}
	if (r.FeeType == "flat" || r.FeeType == "combined") && r.FlatFee == nil {
		return errors.New("flat_fee is required for flat and combined fee types")
	}
	if (r.FeeType == "percentage" || r.FeeType == "combined") && r.PercentageFee == nil {
		return errors.New("percentage_fee is required for percentage and combined fee types")
	}
	if r.FlatFee != nil && *r.FlatFee < 0 {
		return errors.New("flat_fee must be non-negative")
	}
	if r.PercentageFee != nil && (*r.PercentageFee < 0 || *r.PercentageFee > 1) {
		return errors.New("percentage_fee must be between 0 and 1")
	}
	return nil
}

func (a *API) CreateFeeTierHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateFeeTierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	tier := &core.FeeTier{
		TransactionType: req.TransactionType,
		MinAmount:       req.MinAmount,
		MaxAmount:       req.MaxAmount,
		FeeType:         core.FeeType(req.FeeType),
		FlatFee:         req.FlatFee,
		PercentageFee:   req.PercentageFee,
		IsActive:        true,
	}

	createdTier, err := a.service.CreateFeeTier(ctx, tier)
	if err != nil {
		a.logger.Error("failed to create fee tier", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to create fee tier")
		return
	}

	jsonResponse(w, http.StatusCreated, createdTier)
}

func (a *API) ListFeeTiersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	transactionType := r.URL.Query().Get("transaction_type")
	activeOnly := r.URL.Query().Get("active_only") == "true"

	var txType *string
	if transactionType != "" {
		if transactionType != "transfer" && transactionType != "withdraw" {
			httpError(w, http.StatusBadRequest, "transaction_type must be 'transfer' or 'withdraw'")
			return
		}
		txType = &transactionType
	}

	tiers, err := a.service.ListFeeTiers(ctx, txType, activeOnly)
	if err != nil {
		a.logger.Error("failed to list fee tiers", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to list fee tiers")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"fee_tiers": tiers})
}

type UpdateFeeTierRequest struct {
	TransactionType string   `json:"transaction_type"`
	MinAmount       int64    `json:"min_amount"`
	MaxAmount       *int64   `json:"max_amount"`
	FeeType         string   `json:"fee_type"`
	FlatFee         *int64   `json:"flat_fee"`
	PercentageFee   *float64 `json:"percentage_fee"`
	IsActive        bool     `json:"is_active"`
}

func (r *UpdateFeeTierRequest) Validate() error {
	if r.TransactionType != "transfer" && r.TransactionType != "withdraw" {
		return errors.New("transaction_type must be 'transfer' or 'withdraw'")
	}
	if r.MinAmount < 0 {
		return errors.New("min_amount must be non-negative")
	}
	if r.MaxAmount != nil && *r.MaxAmount <= r.MinAmount {
		return errors.New("max_amount must be greater than min_amount")
	}
	if r.FeeType != "flat" && r.FeeType != "percentage" && r.FeeType != "combined" {
		return errors.New("fee_type must be 'flat', 'percentage', or 'combined'")
	}
	if (r.FeeType == "flat" || r.FeeType == "combined") && r.FlatFee == nil {
		return errors.New("flat_fee is required for flat and combined fee types")
	}
	if (r.FeeType == "percentage" || r.FeeType == "combined") && r.PercentageFee == nil {
		return errors.New("percentage_fee is required for percentage and combined fee types")
	}
	if r.FlatFee != nil && *r.FlatFee < 0 {
		return errors.New("flat_fee must be non-negative")
	}
	if r.PercentageFee != nil && (*r.PercentageFee < 0 || *r.PercentageFee > 1) {
		return errors.New("percentage_fee must be between 0 and 1")
	}
	return nil
}

func (a *API) UpdateFeeTierHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpError(w, http.StatusBadRequest, "invalid fee tier id")
		return
	}

	var req UpdateFeeTierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	tier := &core.FeeTier{
		ID:              id,
		TransactionType: req.TransactionType,
		MinAmount:       req.MinAmount,
		MaxAmount:       req.MaxAmount,
		FeeType:         core.FeeType(req.FeeType),
		FlatFee:         req.FlatFee,
		PercentageFee:   req.PercentageFee,
		IsActive:        req.IsActive,
	}

	if err := a.service.UpdateFeeTier(ctx, tier); err != nil {
		if errors.Is(err, storage.ErrFeeRuleNotFound) {
			httpError(w, http.StatusNotFound, "fee tier not found")
			return
		}
		a.logger.Error("failed to update fee tier", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to update fee tier")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "fee tier updated successfully"})
}

func (a *API) DeleteFeeTierHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpError(w, http.StatusBadRequest, "invalid fee tier id")
		return
	}

	if err := a.service.DeleteFeeTier(ctx, id); err != nil {
		if errors.Is(err, storage.ErrFeeRuleNotFound) {
			httpError(w, http.StatusNotFound, "fee tier not found")
			return
		}
		a.logger.Error("failed to delete fee tier", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to delete fee tier")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "fee tier deleted successfully"})
}

func (a *API) CalculateFeeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	transactionType := r.URL.Query().Get("transaction_type")
	amountStr := r.URL.Query().Get("amount")

	if transactionType == "" || amountStr == "" {
		httpError(w, http.StatusBadRequest, "transaction_type and amount are required")
		return
	}

	if transactionType != "transfer" && transactionType != "withdraw" {
		httpError(w, http.StatusBadRequest, "transaction_type must be 'transfer' or 'withdraw'")
		return
	}

	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil || amount <= 0 {
		httpError(w, http.StatusBadRequest, "invalid amount")
		return
	}

	feeCalc, err := a.service.CalculateFee(ctx, transactionType, amount)
	if err != nil {
		a.logger.Error("failed to calculate fee", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to calculate fee")
		return
	}

	jsonResponse(w, http.StatusOK, feeCalc)
}

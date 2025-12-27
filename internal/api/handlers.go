package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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

type getAccountsResponse struct {
	Accounts []*getAccountResponse `json:"accounts"`
}

type transferResponse struct {
	FromAccount *getAccountResponse `json:"from_account"`
	ToAccount   *getAccountResponse `json:"to_account"`
	Reference   string              `json:"reference,omitempty"`
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
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
}

type WithdrawResponse struct {
	Balance   int64  `json:"balance"`
	Reference string `json:"reference"`
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

	resp := getAccountResponse{
		ID:             acc.ID,
		UserID:         acc.UserID,
		Balance:        acc.Balance,
		OverdraftLimit: acc.OverdraftLimit,
		CreatedAt:      acc.CreatedAt,
	}

	jsonResponse(w, http.StatusOK, resp)
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
		accountsResponse = append(accountsResponse, &getAccountResponse{
			ID:             acc.ID,
			UserID:         acc.UserID,
			Balance:        acc.Balance,
			OverdraftLimit: acc.OverdraftLimit,
			CreatedAt:      acc.CreatedAt,
		})
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

	resp := getAccountResponse{
		ID:             updatedAccount.ID,
		UserID:         updatedAccount.UserID,
		Balance:        updatedAccount.Balance,
		OverdraftLimit: updatedAccount.OverdraftLimit,
		CreatedAt:      updatedAccount.CreatedAt,
	}

	jsonResponse(w, http.StatusOK, resp)
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

	fromAcc, toAcc, err := a.service.Transfer(ctx, req.FromID, req.ToID, req.Amount, reference)
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
		FromAccount: &getAccountResponse{
			ID:             fromAcc.ID,
			UserID:         fromAcc.UserID,
			Balance:        fromAcc.Balance,
			OverdraftLimit: fromAcc.OverdraftLimit,
			CreatedAt:      fromAcc.CreatedAt,
		},
		ToAccount: &getAccountResponse{
			ID:             toAcc.ID,
			UserID:         toAcc.UserID,
			Balance:        toAcc.Balance,
			OverdraftLimit: toAcc.OverdraftLimit,
			CreatedAt:      toAcc.CreatedAt,
		},
		Reference: reference,
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

	resp := getAccountResponse{
		ID:             paymentResp.ID,
		UserID:         paymentResp.UserID,
		Balance:        paymentResp.Balance,
		OverdraftLimit: paymentResp.OverdraftLimit,
		CreatedAt:      paymentResp.CreatedAt,
	}
	jsonResponse(w, http.StatusOK, resp)
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

	response, err := a.service.ListTransactions(ctx, accountID)
	if err != nil {
		a.logger.Error("failed to list transactions", "err", err)
		httpError(w, http.StatusInternalServerError, "could not retrieve transactions")
		return
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

	tokenString, err := a.generateJWTToken(resp.ID)
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
	resp, err := a.service.GetUsers(ctx)
	if err != nil {
		a.logger.Error("failed to get users", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to retrieve users")
		return
	}

	var users []*usersResponse
	for _, user := range resp {
		users = append(users, &usersResponse{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		})
	}

	jsonResponse(w, http.StatusOK, users)
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

	token, err := a.generateJWTToken(data.ID)
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

	var request RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request")
		return
	}

	key := fmt.Sprintf("session:%s", request.RefreshToken)
	userIDstr, err := a.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		httpError(w, http.StatusUnauthorized, "invalid token")
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

	if err := a.redis.Del(ctx, key).Err(); err != nil {
		a.logger.Error("failed to delete old refresh token", "err", err)
	}

	newToken, err := a.generateJWTToken(userID)
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

func (a *API) generateJWTToken(userID int) (string, error) {
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(time.Minute * 10).Unix(),
			"app":     "mini-bank",
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

	accountID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check if transaction with this reference already exists (idempotency)
	existingTx, err := a.service.GetTransaction(ctx, req.Reference)
	if err == nil {
		// Transaction already exists - return current balance (idempotent behavior)
		account, err := a.service.GetAccount(ctx, accountID)
		if err != nil {
			a.logger.Error("failed to get account for existing transaction", "err", err)
			httpError(w, http.StatusInternalServerError, "failed to process withdrawal")
			return
		}
		res := WithdrawResponse{
			Balance:   account.Balance,
			Reference: existingTx.Reference,
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

	response, err := a.service.Withdraw(ctx, accountID, req.Amount, req.Reference)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Withdrawal failed")
		return
	}
	res := WithdrawResponse{
		Balance:   response.Balance,
		Reference: req.Reference,
	}
	jsonResponse(w, http.StatusOK, res)
}

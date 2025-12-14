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
)

type API struct {
	service   service.Service
	logger    *slog.Logger
	redis     *redis.Client
	jwtSecret string
}

func NewAPI(s service.Service, logger *slog.Logger, rdb *redis.Client, jwtSecret string) *API {
	return &API{service: s, logger: logger, redis: rdb, jwtSecret: jwtSecret}
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type createAccountRequest struct {
	UserID         int   `json:"user_id"`
	InitialBalance int64 `json:"initial_balance"`
}

type createAccountResponse struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type getAccountResponse struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
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
	Token string `json:"token"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponse struct {
	Token string `json:"token"`
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
		ID:        acc.ID,
		UserID:    acc.UserID,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt,
	}

	jsonResponse(w, http.StatusCreated, resp)
}




func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
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
		ID:        acc.ID,
		UserID:    acc.UserID,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt,
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (a *API) GetAccountsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := a.service.ListAccounts(ctx)
	if err != nil {
		a.logger.Error("failed to get accounts", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to get accounts")
		return
	}

	var accountsResponse []*getAccountResponse
	
	userID, ok := ctx.Value(contextKeyUserID).(int)
	if !ok {
		httpError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	for _, acc := range accounts {
		if acc.UserID != userID {
			continue
		}
		accountsResponse = append(accountsResponse, &getAccountResponse{
			ID:        acc.ID,
			UserID:    acc.UserID,
			Balance:   acc.Balance,
			CreatedAt: acc.CreatedAt,
		})
	}

	resp := getAccountsResponse{
		Accounts: accountsResponse,
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
			ID:        fromAcc.ID,
			UserID:    fromAcc.UserID,
			Balance:   fromAcc.Balance,
			CreatedAt: fromAcc.CreatedAt,
		},
		ToAccount: &getAccountResponse{
			ID:        toAcc.ID,
			UserID:    toAcc.UserID,
			Balance:   toAcc.Balance,
			CreatedAt: toAcc.CreatedAt,
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
		ID:        paymentResp.ID,
		UserID:    paymentResp.UserID,
		Balance:   paymentResp.Balance,
		CreatedAt: paymentResp.CreatedAt,
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

	data, err := a.service.Login(ctx, request.Email, request.Password)
	if err != nil {
		// We log the actual error for debugging but return a generic message to the user
		a.logger.Warn("login failed", "email", request.Email, "err", err)
		httpError(w, http.StatusUnauthorized, "Invalid email or password")
		return
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

	// Invalidate the old refresh token to prevent reuse
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

	return token, nil
}

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"mini-bank/internal/storage"

	"github.com/google/uuid"
)

type API struct {
	store  storage.Storage
	logger *slog.Logger
}

func NewAPI(s storage.Storage, logger *slog.Logger) *API {
	return &API{store: s, logger: logger}
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type createAccountRequest struct {
	Name           string  `json:"name"`
	InitialBalance float64 `json:"initial_balance"`
}

type createAccountResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type getAccountResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Balance   float64   `json:"balance"`
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
	FromID int     `json:"from_id"`
	ToID   int     `json:"to_id"`
	Amount float64 `json:"amount"`
}

type paymentRequest struct {
	AccountID int                 `json:"account_id"`
	Amount    float64             `json:"amount"`
	Type      storage.PaymentType `json:"type"`
}

func (a *API) CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := validateCreateAccount(req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	acc, err := a.store.CreateAccount(ctx, req.Name, req.InitialBalance)
	if err != nil {
		a.logger.Error("failed to create account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to create account")

		return
	}

	resp := createAccountResponse{
		ID:        acc.ID,
		Name:      acc.Name,
		Balance:   acc.Balance,
		CreatedAt: acc.CreatedAt,
	}

	jsonResponse(w, http.StatusCreated, resp)
}

func validateCreateAccount(req createAccountRequest) error {
	if req.Name == "" {
		return errors.New("name is required")
	}

	if req.InitialBalance < 0 {
		return errors.New("initial balance must be positive")
	}

	return nil
}

func validateTransferRequest(req transferRequest) error {
	if req.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if req.FromID == req.ToID {
		return errors.New("sender and receiver accounts cannot be the same")
	}
	if req.FromID <= 0 {
		return errors.New("invalid sender account id")
	}
	if req.ToID <= 0 {
		return errors.New("invalid receiver account id")
	}
	return nil
}

func validatePaymentRequest(req paymentRequest) error {
	if req.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if req.AccountID <= 0 {
		return errors.New("account ID must be greater than zero")
	}
	return nil
}

func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (a *API) GetAccountHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	ctx := r.Context()
	acc, err := a.store.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrAccountNotFound) {
			httpError(w, http.StatusNotFound, "account not found")
			return
		}
		a.logger.Error("failed to get account", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to get account")
		return

	}

	resp := getAccountResponse{
		ID: acc.ID,
		Name: acc.Name,
		Balance: acc.Balance,
		CreatedAt: acc.CreatedAt,
	}

	jsonResponse(w, http.StatusOK, resp)
}

func (a *API) GetAccountsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := a.store.ListAccounts(ctx)
	if err != nil {
		a.logger.Error("failed to get accounts", "err", err)
		httpError(w, http.StatusInternalServerError, "failed to get accounts")
		return
	}

	var accountsResponse []*getAccountResponse

	for _, acc := range accounts {
		accountsResponse = append(accountsResponse, &getAccountResponse{
			ID: acc.ID,
			Name: acc.Name,
			Balance: acc.Balance,
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

	if err := validateTransferRequest(req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	reference := uuid.NewString()

	fromAcc, toAcc, err := a.store.Transfer(ctx, req.FromID, req.ToID, req.Amount, reference)
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
			ID: fromAcc.ID,
			Name: fromAcc.Name,
			Balance: fromAcc.Balance,
			CreatedAt: fromAcc.CreatedAt,
		},
		ToAccount: &getAccountResponse{
			ID: toAcc.ID,
			Name: toAcc.Name,
			Balance: toAcc.Balance,
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
	if err := validatePaymentRequest(req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	reference := uuid.NewString()

	paymentResp, err := a.store.Payment(ctx, req.AccountID, req.Amount, req.Type, reference)
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
		ID: paymentResp.ID,
		Name: paymentResp.Name,
		Balance: paymentResp.Balance,
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

	response, err := a.store.ListTransactions(ctx, accountID)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, response)
}

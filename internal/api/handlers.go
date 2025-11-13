package api

import (
	"encoding/json"
	"errors"
	"mini-bank/internal/storage"
	"net/http"
	"path"
	"strconv"
)

type API struct {
	store storage.Storage
}

func NewAPI(s storage.Storage) *API {
	return &API{store: s}
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
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

type getAccountResponse struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
}

type getAccountsResponse struct {
	Accounts []*getAccountResponse `json:"accounts"`
}

type transferRequest struct {
	FromID int     `json:"from_id"`
	ToID   int     `json:"to_id"`
	Amount float64 `json:"amount"`
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
		httpError(w, http.StatusInternalServerError, "failed to create account")

		return
	}

	resp := createAccountResponse{
		ID:      acc.ID,
		Name:    acc.Name,
		Balance: acc.Balance,
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

func validateDepositRequest(req transferRequest) error {
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

func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (a *API) GetAccountHandler(w http.ResponseWriter, r *http.Request) {

	idStr := path.Base(r.URL.Path)
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		httpError(w, http.StatusBadRequest, "invalid account id")
		return
	}

	ctx := r.Context()
	acc, err := a.store.GetAccount(ctx, id)
	if err != nil {
		httpError(w, http.StatusNotFound, "account not found")
		return
	}

	jsonResponse(w, http.StatusOK, acc)
}

func (a *API) GetAccountsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accounts, err := a.store.ListAccounts(ctx)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "failed to get accounts")
		return
	}

	var accountsResponse []*getAccountResponse

	for _, acc := range accounts {
		accountsResponse = append(accountsResponse, &getAccountResponse{
			ID:      acc.ID,
			Name:    acc.Name,
			Balance: acc.Balance,
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

	if err := validateDepositRequest(req); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}

	senderID := req.FromID
	receiverID := req.ToID
	amount := req.Amount
	err := a.store.Transfer(ctx, senderID, receiverID, amount)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "transfer failed")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "transfer successful"})
}

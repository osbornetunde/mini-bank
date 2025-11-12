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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
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

	resp := getAccountResponse{
		ID:      acc.ID,
		Name:    acc.Name,
		Balance: acc.Balance,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
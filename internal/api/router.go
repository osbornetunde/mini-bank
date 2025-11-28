package api

import "net/http"

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	// Account routes
	mux.HandleFunc("POST /api/v1/accounts", a.CreateAccountHandler)
	mux.HandleFunc("GET /api/v1/accounts", a.GetAccountsHandler)
	mux.HandleFunc("GET /api/v1/accounts/{id}", a.GetAccountHandler)

	// Transaction routes
	mux.HandleFunc("POST /api/v1/transactions/transfer", a.TransferHandler)
	mux.HandleFunc("POST /api/v1/transactions/payment", a.PaymentHandler)
	mux.HandleFunc("GET /api/v1/accounts/{id}/transactions", a.GetTransactionsHandler)
	mux.HandleFunc("GET /api/v1/transactions/{ref}", a.GetTransactionHandler)
	
	// User routes
	mux.HandleFunc("POST  /api/v1/users/create", a.CreateUserHandler)
	mux.HandleFunc("GET /api/v1/users", a.GetUsersHandler)
	mux.HandleFunc("GET /api/v1/users/{id}", a.GetUserHandler)
	mux.HandleFunc("PUT /api/v1/users/{id}", a.UpdateUserHandler)

	return mux
}

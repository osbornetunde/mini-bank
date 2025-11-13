package api

import (
	"net/http"
	"strings"
)

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			a.GetAccountsHandler(w, r)
		case http.MethodPost:
			a.CreateAccountHandler(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/accounts/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/accounts/")
		idStr = strings.TrimSuffix(idStr, "/")

		if idStr == "" {
			http.Error(w, "account id required", http.StatusBadRequest)
			return
		}

		if strings.Contains(idStr, "/") {
			http.NotFound(w, r)
			return
		}

		a.GetAccountHandler(w, r)
	})

	mux.HandleFunc("/transactions/transfer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		a.TransferHandler(w, r)
	})

	return mux
}

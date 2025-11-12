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

		trimmed := strings.TrimPrefix(r.URL.Path, "/accounts/")
		if trimmed == "" || trimmed == "/" {
			http.Error(w, "account id required", http.StatusBadRequest)
			return
		}
		a.GetAccountHandler(w, r)
	})

	return mux
}

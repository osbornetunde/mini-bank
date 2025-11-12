package api

import (
	"net/http"
)

func (a *API) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			a.CreateAccountHandler(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	return mux
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"mini-bank/internal/api"
	mem "mini-bank/internal/storage/memory"
)

func main() {
	// choose storage backend
	store := mem.NewStore() // in-memory for now

	// create API and router
	a := api.NewAPI(store)
	handler := a.Router()

	// http server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// run server in goroutine
	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}

	log.Println("server stopped")
}

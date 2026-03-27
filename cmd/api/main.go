package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/your-org/ledger-engine/internal/auth"
	"github.com/your-org/ledger-engine/internal/db"
	"github.com/your-org/ledger-engine/internal/handlers"
	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/models"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.New(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	ledgerService := &ledger.Service{
		DB: database.DB,
	}
	authResolver := auth.NewWorkOSAuth()
	// Print startup warnings for common env mistakes (wrong client id, weak cookie secret, etc).
	for _, warning := range authResolver.ConfigWarnings() {
		log.Printf("auth config warning: %s", warning)
	}

	h := &handlers.Handler{
		DB:           database.DB,
		Ledger:       ledgerService,
		AuthProvider: authResolver,
	}

	r := chi.NewRouter()

	// Basic health endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", h.AuthConsolePage)

	// Users
	r.Post("/users", h.CreateUser)

	// Auth flow routes.
	// These are public because they bootstrap browser authentication.
	r.Get("/auth/login", h.AuthLogin)
	r.Get("/auth/callback", h.AuthCallback)
	r.Get("/auth/status", h.AuthStatus)
	r.Post("/auth/logout", h.AuthLogout)

	// Authenticated routes (WorkOS session cookie or bearer token).
	userModel := &models.UserModel{DB: database.DB}
	r.Group(func(pr chi.Router) {
		pr.Use(auth.Middleware(userModel, authResolver))
		pr.Get("/auth/me", h.Me)
		pr.Post("/accounts", h.CreateAccount)
		pr.Get("/accounts/{id}/balance", h.GetAccountBalance)
		pr.Post("/transactions", h.CreateTransaction)
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("ledger-engine listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown error: %v", err)
	}
}

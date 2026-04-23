package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/go-chi/chi/v5"

	"github.com/your-org/ledger-engine/internal/auth"
	"github.com/your-org/ledger-engine/internal/db"
	"github.com/your-org/ledger-engine/internal/handlers"
	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/middleware"
	"github.com/your-org/ledger-engine/internal/models"
)

func main() {

	if err := godotenv.Load(); err != nil {
		if err := godotenv.Load("../../.env"); err != nil {
			log.Println("No .env file found")
		}
	}
	if err := validateRuntimeConfig(); err != nil {
		log.Fatalf("invalid runtime configuration: %v", err)
	}
	// rest of your main function
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
	metrics := middleware.NewMetrics()
	r.Use(middleware.RequestID)
	r.Use(metrics.Middleware)
	if rateLimiter := middleware.NewRateLimiterFromEnv(); rateLimiter != nil {
		r.Use(rateLimiter.Middleware)
	}

	// Basic health endpoint
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/graphql/schema", h.GraphQLSchema)
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := database.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "not-ready",
				"error":  "database unavailable",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ready",
		})
	})
	r.Handle("/metrics", metrics.Handler())
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
		pr.Post("/graphql", h.GraphQL)
		pr.Post("/accounts", h.CreateAccount)
		pr.Get("/accounts/{id}/balance", h.GetAccountBalance)
		pr.Post("/transactions", h.CreateTransaction)
		pr.Post("/bootstrap/demo", h.BootstrapDemo)
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

func validateRuntimeConfig() error {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" {
		appEnv = "development"
	}

	cookieSecret := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECRET"))
	if len(cookieSecret) < 32 {
		return errors.New("SESSION_COOKIE_SECRET must be at least 32 characters")
	}

	if appEnv == "production" {
		internalServiceToken := strings.TrimSpace(os.Getenv("INTERNAL_SERVICE_TOKEN"))
		if internalServiceToken == "" {
			return errors.New("INTERNAL_SERVICE_TOKEN is required in production")
		}

		kmsKey := strings.TrimSpace(os.Getenv("KMS_MASTER_KEY_B64"))
		if kmsKey == "" {
			return errors.New("KMS_MASTER_KEY_B64 is required in production")
		}

		decoded, err := base64.StdEncoding.DecodeString(kmsKey)
		if err != nil || len(decoded) != 32 {
			return errors.New("KMS_MASTER_KEY_B64 must be valid base64 for a 32-byte AES-256 key")
		}
	}

	return nil
}

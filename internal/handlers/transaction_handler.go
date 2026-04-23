package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/auth"
	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/models"
)

type Handler struct {
	DB     *sql.DB
	Ledger *ledger.Service
	// AuthProvider handles WorkOS login, callback exchange, and cookie sessions.
	AuthProvider *auth.WorkOSAuth
}

type createUserRequest struct {
	Email  string `json:"email"`
	Status string `json:"status"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Status == "" {
		http.Error(w, "email and status are required", http.StatusBadRequest)
		return
	}

	userModel := &models.UserModel{DB: h.DB}
	user, err := userModel.Insert(ctx, req.Email, req.Status)
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

type createAccountRequest struct {
	Name string             `json:"name"`
	Type models.AccountType `json:"type"`
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser, ok := auth.UserFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Type == "" {
		http.Error(w, "name and type are required", http.StatusBadRequest)
		return
	}

	accountModel := &models.AccountModel{DB: h.DB}
	account, err := accountModel.Insert(ctx, authUser.ID, req.Name, req.Type)
	if err != nil {
		http.Error(w, "failed to create account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(account)
}

type createTransactionRequest struct {
	Source            string                  `json:"source"`
	ExternalReference string                  `json:"external_reference,omitempty"`
	Description       string                  `json:"description,omitempty"`
	Lines             []createTransactionLine `json:"lines"`
}

type createTransactionLine struct {
	AccountID string `json:"account_id"`
	Debit     int64  `json:"debit"`
	Credit    int64  `json:"credit"`
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser, ok := auth.UserFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Source == "" || len(req.Lines) < 2 {
		http.Error(w, "source and at least two lines are required", http.StatusBadRequest)
		return
	}

	lines := make([]ledger.LineInput, 0, len(req.Lines))
	for _, l := range req.Lines {
		accountID, err := uuid.Parse(l.AccountID)
		if err != nil {
			http.Error(w, "invalid account_id in lines", http.StatusBadRequest)
			return
		}
		lines = append(lines, ledger.LineInput{
			AccountID: accountID,
			Debit:     l.Debit,
			Credit:    l.Credit,
		})
	}

	input := ledger.CreateTransactionInput{
		UserID:            authUser.ID,
		Source:            req.Source,
		ExternalReference: req.ExternalReference,
		Description:       req.Description,
		Lines:             lines,
	}

	txn, entries, err := h.Ledger.CreateTransaction(ctx, input)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ledger.ErrInvalidLines),
			errors.Is(err, ledger.ErrUnbalancedTransaction),
			errors.Is(err, ledger.ErrUnauthorizedAccount):
			status = http.StatusBadRequest
		}

		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"transaction": txn,
		"entries":     entries,
	})
}

func (h *Handler) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authUser, ok := auth.UserFromContext(ctx)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		http.Error(w, "missing account id", http.StatusBadRequest)
		return
	}
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid account id", http.StatusBadRequest)
		return
	}

	accountModel := &models.AccountModel{DB: h.DB}
	account, err := accountModel.GetByID(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load account", http.StatusInternalServerError)
		return
	}
	if account.UserID != authUser.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	balance, err := h.Ledger.GetAccountBalance(ctx, accountID)
	if err != nil {
		http.Error(w, "failed to compute balance", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"account_id": accountID,
		"balance":    balance,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// test commit

func (h *Handler) AuthLogin(w http.ResponseWriter, r *http.Request) {
	domain := os.Getenv("WORKOS_AUTHKIT_DOMAIN")
	clientID := os.Getenv("WORKOS_CLIENT_ID")
	redirectURI := os.Getenv("WORKOS_REDIRECT_URI")

	if domain == "" || clientID == "" || redirectURI == "" {
		http.Error(w, "Missing WorkOS config", http.StatusInternalServerError)
		return
	}

	authURL := fmt.Sprintf(
		"https://%s/?client_id=%s&redirect_uri=%s",
		domain,
		clientID,
		url.QueryEscape(redirectURI),
	)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *Handler) AuthCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("🔥 HIT AUTH CALLBACK")

	code := r.URL.Query().Get("code")
	if code == "" {
		log.Println("❌ missing code")
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	accessToken, err := h.AuthProvider.ExchangeCode(r.Context(), code)
	if err != nil {
		log.Printf("❌ ExchangeCode error: %v", err)
		http.Error(w, "could not exchange authorization code", http.StatusUnauthorized)
		return
	}

	sessionCookie, err := h.AuthProvider.EncodeSession(accessToken, 24*time.Hour)
	if err != nil {
		http.Error(w, "could not generate session cookie", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionCookie,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) AuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AuthStatus(w http.ResponseWriter, r *http.Request) {
	// Lightweight endpoint for frontend polling/UI checks.
	user, ok := h.resolveAuthUserOptional(r)
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authenticated": false,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"authenticated": true,
		"user":          user,
	})
}

func (h *Handler) AuthConsolePage(w http.ResponseWriter, r *http.Request) {
	// Minimal in-app page to manually test login/logout/status behavior.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <title>Ledger Auth Console</title>
    <style>
      body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; max-width: 720px; }
      .row { display: flex; gap: 8px; margin-bottom: 16px; }
      button { padding: 10px 14px; border: 1px solid #ccc; border-radius: 8px; background: #fff; cursor: pointer; }
      pre { background: #f5f5f5; border-radius: 8px; padding: 12px; overflow: auto; }
    </style>
  </head>
  <body>
    <h1>Ledger Auth Console</h1>
    <p>Use this page to test login/logout and auth status.</p>
    <div class="row">
      <button id="login">Login</button>
      <button id="logout">Logout</button>
      <button id="refresh">Refresh Status</button>
    </div>
    <pre id="status">Loading...</pre>
    <script>
      async function refreshStatus() {
        const res = await fetch("/auth/status", { credentials: "include" });
        const data = await res.json();
        document.getElementById("status").textContent = JSON.stringify(data, null, 2);
      }
      document.getElementById("login").onclick = () => { window.location.href = "/auth/login"; };
      document.getElementById("logout").onclick = async () => {
        await fetch("/auth/logout", { method: "POST", credentials: "include" });
        await refreshStatus();
      };
      document.getElementById("refresh").onclick = refreshStatus;
      refreshStatus();
    </script>
  </body>
</html>`))
}

func (h *Handler) resolveAuthUserOptional(r *http.Request) (*models.User, bool) {
	// Best-effort user resolution for status endpoint (no hard errors on missing auth).
	if h.AuthProvider == nil {
		return nil, false
	}

	var token string
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	if token == "" {
		cookie, err := r.Cookie("session")
		if err == nil && cookie != nil {
			token, _ = h.AuthProvider.DecodeSession(cookie.Value)
		}
	}
	if token == "" {
		return nil, false
	}

	email, err := h.AuthProvider.ResolveEmail(r.Context(), token)
	if err != nil {
		return nil, false
	}

	userModel := &models.UserModel{DB: h.DB}
	user, err := userModel.GetByEmail(r.Context(), email)
	if errors.Is(err, sql.ErrNoRows) {
		user, err = userModel.Insert(r.Context(), email, "active")
	}
	if err != nil {
		return nil, false
	}

	return user, true
}

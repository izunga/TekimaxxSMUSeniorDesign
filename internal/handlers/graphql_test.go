package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	ledgerauth "github.com/your-org/ledger-engine/internal/auth"
	"github.com/your-org/ledger-engine/internal/handlers"
	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/models"
	"github.com/your-org/ledger-engine/internal/testutil"
)

func TestGraphQLQueryMutationAndAuthorization(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	t.Setenv("SESSION_COOKIE_SECRET", "supersecurestringthatisatleast32characterslong")
	t.Setenv("INTERNAL_SERVICE_TOKEN", "graphql-test-token")
	t.Setenv("INTERNAL_SERVICE_EMAIL", "graphql-service@example.com")

	router := newGraphQLTestRouter(db)

	queryResp := doGraphQLRequest(t, router, "graphql-test-token", map[string]any{
		"operationName": "Me",
		"query":         "query Me { me { id email status } }",
		"variables":     map[string]any{},
	})
	if queryResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", queryResp.Code, queryResp.Body.String())
	}

	var mePayload map[string]map[string]map[string]any
	if err := json.Unmarshal(queryResp.Body.Bytes(), &mePayload); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if mePayload["data"]["me"]["email"] != "graphql-service@example.com" {
		t.Fatalf("unexpected me email: %v", mePayload["data"]["me"]["email"])
	}

	mutationResp := doGraphQLRequest(t, router, "graphql-test-token", map[string]any{
		"operationName": "CreateAccount",
		"query":         "mutation CreateAccount($input: CreateAccountInput!) { createAccount(input: $input) { id name type } }",
		"variables": map[string]any{
			"input": map[string]any{
				"name": "GraphQL Cash",
				"type": "asset",
			},
		},
	})
	if mutationResp.Code != http.StatusOK {
		t.Fatalf("expected 200 mutation response, got %d: %s", mutationResp.Code, mutationResp.Body.String())
	}

	serviceUser := loadUserByEmail(t, db, "graphql-service@example.com")
	otherUser := createUser(t, db, "other-user@example.com")
	otherAccount := createAccount(t, db, otherUser.ID, "Other Cash", models.AccountTypeAsset)

	authzResp := doGraphQLRequest(t, router, "graphql-test-token", map[string]any{
		"operationName": "AccountBalance",
		"query":         "query AccountBalance($accountId: ID!) { accountBalance(accountId: $accountId) { account_id balance } }",
		"variables": map[string]any{
			"accountId": otherAccount.ID.String(),
		},
	})
	if authzResp.Code != http.StatusNotFound {
		t.Fatalf("expected authorization failure status 404, got %d: %s", authzResp.Code, authzResp.Body.String())
	}

	validationResp := doGraphQLRequest(t, router, "graphql-test-token", map[string]any{
		"operationName": "CreateTransaction",
		"query":         "mutation CreateTransaction($input: CreateTransactionInput!) { createTransaction(input: $input) { transaction { id } } }",
		"variables": map[string]any{
			"input": map[string]any{
				"lines": []map[string]any{},
			},
		},
	})
	if validationResp.Code != http.StatusBadRequest {
		t.Fatalf("expected validation failure status 400, got %d: %s", validationResp.Code, validationResp.Body.String())
	}

	_ = serviceUser
}

func newGraphQLTestRouter(db *sql.DB) http.Handler {
	authResolver := ledgerauth.NewWorkOSAuth()
	handler := &handlers.Handler{
		DB:           db,
		Ledger:       &ledger.Service{DB: db},
		AuthProvider: authResolver,
	}

	router := chi.NewRouter()
	userModel := &models.UserModel{DB: db}
	router.Group(func(r chi.Router) {
		r.Use(ledgerauth.Middleware(userModel, authResolver))
		r.Post("/graphql", handler.GraphQL)
	})
	return router
}

func doGraphQLRequest(t *testing.T, handler http.Handler, token string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func loadUserByEmail(t *testing.T, db *sql.DB, email string) *models.User {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userModel := &models.UserModel{DB: db}
	user, err := userModel.GetByEmail(ctx, email)
	if err != nil {
		t.Fatalf("load user by email: %v", err)
	}
	return user
}

func createUser(t *testing.T, db *sql.DB, email string) *models.User {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userModel := &models.UserModel{DB: db}
	user, err := userModel.Insert(ctx, email, "active")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func createAccount(t *testing.T, db *sql.DB, userID uuid.UUID, name string, accountType models.AccountType) *models.Account {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accountModel := &models.AccountModel{DB: db}
	account, err := accountModel.Insert(ctx, userID, name, accountType)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/auth"
	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/middleware"
	"github.com/your-org/ledger-engine/internal/models"
)

type graphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type graphQLResponse struct {
	Data   any             `json:"data,omitempty"`
	Errors []graphQLError  `json:"errors,omitempty"`
}

type graphQLAccount struct {
	models.Account
	Balance int64 `json:"balance"`
}

func (h *Handler) GraphQLSchema(w http.ResponseWriter, r *http.Request) {
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") {
		h.writeGraphQLError(w, http.StatusNotFound, "not found", "NOT_FOUND")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"endpoint": "/graphql",
		"supported_operations": []string{
			"Me",
			"Accounts",
			"AccountBalance",
			"Transactions",
			"AuditLogs",
			"CreateAccount",
			"CreateTransaction",
			"BootstrapDemo",
		},
		"notes": []string{
			"Send a standard GraphQL JSON body with query, operationName, and variables.",
			"This is a focused GraphQL adapter over the ledger service for demo use.",
		},
	})
}

func (h *Handler) GraphQL(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		h.writeGraphQLError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req graphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeGraphQLError(w, http.StatusBadRequest, "invalid GraphQL request", "BAD_REQUEST")
		return
	}
	if strings.TrimSpace(req.OperationName) == "" {
		h.writeGraphQLError(w, http.StatusBadRequest, "operationName is required", "BAD_REQUEST")
		return
	}
	if strings.Contains(req.Query, "__schema") || strings.Contains(req.Query, "__type") {
		h.writeGraphQLError(w, http.StatusForbidden, "introspection is disabled", "FORBIDDEN")
		return
	}

	operation := detectGraphQLOperation(req)
	if operation == "" {
		h.writeGraphQLError(w, http.StatusBadRequest, "unsupported GraphQL operation", "BAD_REQUEST")
		return
	}

	data, status, err := h.executeGraphQLOperation(r, user, operation, req.Variables)
	if err != nil {
		code := "INTERNAL"
		if status >= 400 && status < 500 {
			code = "BAD_REQUEST"
		}
		h.writeGraphQLError(w, status, err.Error(), code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(graphQLResponse{Data: data})
}

func (h *Handler) executeGraphQLOperation(r *http.Request, user *models.User, operation string, variables map[string]interface{}) (any, int, error) {
	ctx := r.Context()

	switch operation {
	case "Me":
		return map[string]any{"me": user}, http.StatusOK, nil
	case "Accounts":
		accountModel := &models.AccountModel{DB: h.DB}
		accounts, err := accountModel.ListByUser(ctx, user.ID)
		if err != nil {
			return nil, http.StatusInternalServerError, errors.New("failed to load accounts")
		}
		enriched := make([]graphQLAccount, 0, len(accounts))
		for _, account := range accounts {
			balance, err := h.Ledger.GetAccountBalance(ctx, account.ID)
			if err != nil {
				return nil, http.StatusInternalServerError, errors.New("failed to compute account balance")
			}
			enriched = append(enriched, graphQLAccount{Account: account, Balance: balance})
		}
		return map[string]any{"accounts": enriched}, http.StatusOK, nil
	case "AccountBalance":
		accountID, err := uuid.Parse(stringVariable(variables, "accountId", "id"))
		if err != nil {
			return nil, http.StatusBadRequest, errors.New("invalid account id")
		}
		accountModel := &models.AccountModel{DB: h.DB}
		account, err := accountModel.GetByID(ctx, accountID)
		if err != nil || account.UserID != user.ID {
			return nil, http.StatusNotFound, errors.New("account not found")
		}
		balance, err := h.Ledger.GetAccountBalance(ctx, accountID)
		if err != nil {
			return nil, http.StatusInternalServerError, errors.New("failed to compute account balance")
		}
		return map[string]any{"accountBalance": map[string]any{"account_id": accountID, "balance": balance}}, http.StatusOK, nil
	case "Transactions":
		limit := intVariable(variables, "limit")
		txnModel := &models.TransactionModel{DB: h.DB}
		transactions, err := txnModel.ListByUser(ctx, user.ID, limit)
		if err != nil {
			return nil, http.StatusInternalServerError, errors.New("failed to load transactions")
		}
		return map[string]any{"transactions": transactions}, http.StatusOK, nil
	case "AuditLogs":
		limit := intVariable(variables, "limit")
		auditModel := &models.AuditLogModel{DB: h.DB}
		logs, err := auditModel.ListByUser(ctx, user.ID, limit)
		if err != nil {
			return nil, http.StatusInternalServerError, errors.New("failed to load audit logs")
		}
		return map[string]any{"auditLogs": logs}, http.StatusOK, nil
	case "CreateAccount":
		name, accountType := accountInputFromVariables(variables)
		if name == "" || accountType == "" {
			return nil, http.StatusBadRequest, errors.New("name and type are required")
		}
		accountModel := &models.AccountModel{DB: h.DB}
		account, err := accountModel.Insert(ctx, user.ID, name, models.AccountType(accountType))
		if err != nil {
			log.Printf("request_id=%s graphql_create_account failed: %v", middleware.GetRequestID(ctx), err)
			return nil, http.StatusInternalServerError, errors.New("failed to create account")
		}
		return map[string]any{"createAccount": account}, http.StatusCreated, nil
	case "CreateTransaction":
		input, err := transactionInputFromVariables(user.ID, variables)
		if err != nil {
			return nil, http.StatusBadRequest, err
		}
		txn, entries, err := h.Ledger.CreateTransaction(ctx, input)
		if err != nil {
			log.Printf("request_id=%s graphql_create_transaction failed: %v", middleware.GetRequestID(ctx), err)
			status := http.StatusInternalServerError
			if errors.Is(err, ledger.ErrInvalidLines) || errors.Is(err, ledger.ErrUnbalancedTransaction) || errors.Is(err, ledger.ErrUnauthorizedAccount) {
				status = http.StatusBadRequest
			}
			return nil, status, err
		}
		return map[string]any{"createTransaction": map[string]any{"transaction": txn, "entries": entries}}, http.StatusCreated, nil
	case "BootstrapDemo":
		accountModel := &models.AccountModel{DB: h.DB}
		seeds := []struct {
			Name string
			Type models.AccountType
			Env  string
		}{
			{Name: "Stripe Balance", Type: models.AccountTypeAsset, Env: "LEDGER_STRIPE_BALANCE_ACCOUNT_ID"},
			{Name: "Revenue", Type: models.AccountTypeRevenue, Env: "LEDGER_REVENUE_ACCOUNT_ID"},
			{Name: "Contra-Revenue", Type: models.AccountTypeExpense, Env: "LEDGER_CONTRA_REVENUE_ACCOUNT_ID"},
		}
		accounts := make(map[string]any, len(seeds))
		exports := make(map[string]string, len(seeds))
		for _, seed := range seeds {
			account, err := accountModel.GetByUserAndName(ctx, user.ID, seed.Name)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return nil, http.StatusInternalServerError, errors.New("failed to bootstrap accounts")
				}
				account, err = accountModel.Insert(ctx, user.ID, seed.Name, seed.Type)
				if err != nil {
					return nil, http.StatusInternalServerError, errors.New("failed to bootstrap accounts")
				}
			}
			accounts[seed.Name] = account
			exports[seed.Env] = account.ID.String()
		}
		return map[string]any{"bootstrapDemo": map[string]any{"user": user, "accounts": accounts, "exports": exports}}, http.StatusOK, nil
	default:
		return nil, http.StatusBadRequest, errors.New("unsupported GraphQL operation")
	}
}

func detectGraphQLOperation(req graphQLRequest) string {
	if req.OperationName != "" {
		return req.OperationName
	}
	query := strings.ToLower(req.Query)
	switch {
	case strings.Contains(query, "createtransaction"):
		return "CreateTransaction"
	case strings.Contains(query, "createaccount"):
		return "CreateAccount"
	case strings.Contains(query, "bootstrapdemo"):
		return "BootstrapDemo"
	case strings.Contains(query, "accountbalance"):
		return "AccountBalance"
	case strings.Contains(query, "auditlogs"):
		return "AuditLogs"
	case strings.Contains(query, "transactions"):
		return "Transactions"
	case strings.Contains(query, "accounts"):
		return "Accounts"
	case strings.Contains(query, "me"):
		return "Me"
	default:
		return ""
	}
}

func (h *Handler) writeGraphQLError(w http.ResponseWriter, status int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(graphQLResponse{
		Errors: []graphQLError{{Message: message, Code: code}},
	})
}

func stringVariable(variables map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := variables[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intVariable(variables map[string]interface{}, key string) int {
	switch value := variables[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 20
	}
}

func accountInputFromVariables(variables map[string]interface{}) (string, string) {
	if input, ok := variables["input"].(map[string]interface{}); ok {
		return stringVariable(input, "name"), stringVariable(input, "type")
	}
	return stringVariable(variables, "name"), stringVariable(variables, "type")
}

func transactionInputFromVariables(userID uuid.UUID, variables map[string]interface{}) (ledger.CreateTransactionInput, error) {
	input := variables
	if nested, ok := variables["input"].(map[string]interface{}); ok {
		input = nested
	}

	if stringVariable(input, "source") == "" {
		return ledger.CreateTransactionInput{}, errors.New("source and at least two lines are required")
	}

	linesValue, ok := input["lines"].([]interface{})
	if !ok || len(linesValue) < 2 {
		return ledger.CreateTransactionInput{}, errors.New("source and at least two lines are required")
	}

	lines := make([]ledger.LineInput, 0, len(linesValue))
	for _, raw := range linesValue {
		lineMap, ok := raw.(map[string]interface{})
		if !ok {
			return ledger.CreateTransactionInput{}, errors.New("invalid lines")
		}
		accountID, err := uuid.Parse(stringVariable(lineMap, "accountId", "account_id"))
		if err != nil {
			return ledger.CreateTransactionInput{}, errors.New("invalid account id in lines")
		}
		lines = append(lines, ledger.LineInput{
			AccountID: accountID,
			Debit:     int64(intVariable(lineMap, "debit")),
			Credit:    int64(intVariable(lineMap, "credit")),
		})
	}

	return ledger.CreateTransactionInput{
		UserID:            userID,
		Source:            stringVariable(input, "source"),
		ExternalReference: stringVariable(input, "externalReference", "external_reference"),
		Description:       stringVariable(input, "description"),
		Lines:             lines,
	}, nil
}

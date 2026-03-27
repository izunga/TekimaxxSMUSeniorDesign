package ledger_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/models"
	"github.com/your-org/ledger-engine/internal/testutil"
)

// helper to create a user and two accounts for tests
func setupTestAccounts(t *testing.T, db *sql.DB) (userID, debitAccountID, creditAccountID uuid.UUID) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Insert(ctx, "test@example.com", "active")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	accountModel := &models.AccountModel{DB: db}

	debitAccount, err := accountModel.Insert(ctx, user.ID, "Cash", models.AccountTypeAsset)
	if err != nil {
		t.Fatalf("insert debit account: %v", err)
	}

	creditAccount, err := accountModel.Insert(ctx, user.ID, "Revenue", models.AccountTypeRevenue)
	if err != nil {
		t.Fatalf("insert credit account: %v", err)
	}

	return user.ID, debitAccount.ID, creditAccount.ID
}

func TestCreateTransaction_BalancedSuccess(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	userID, debitAccountID, creditAccountID := setupTestAccounts(t, db)

	svc := &ledger.Service{DB: db}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := ledger.CreateTransactionInput{
		UserID:      userID,
		Source:      "manual",
		Description: "Test balanced transaction",
		Lines: []ledger.LineInput{
			{
				AccountID: debitAccountID,
				Debit:     100,
				Credit:    0,
			},
			{
				AccountID: creditAccountID,
				Debit:     0,
				Credit:    100,
			},
		},
	}

	txn, entries, err := svc.CreateTransaction(ctx, input)
	if err != nil {
		t.Fatalf("CreateTransaction error: %v", err)
	}
	if txn == nil {
		t.Fatalf("expected transaction, got nil")
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Verify balances
	debitBal, err := svc.GetAccountBalance(ctx, debitAccountID)
	if err != nil {
		t.Fatalf("GetAccountBalance (debit) error: %v", err)
	}
	if debitBal != 100 {
		t.Fatalf("expected debit account balance 100, got %d", debitBal)
	}

	creditBal, err := svc.GetAccountBalance(ctx, creditAccountID)
	if err != nil {
		t.Fatalf("GetAccountBalance (credit) error: %v", err)
	}
	if creditBal != -100 {
		t.Fatalf("expected credit account balance -100, got %d", creditBal)
	}
}

func TestCreateTransaction_UnbalancedFails(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	userID, debitAccountID, creditAccountID := setupTestAccounts(t, db)

	svc := &ledger.Service{DB: db}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := ledger.CreateTransactionInput{
		UserID:      userID,
		Source:      "manual",
		Description: "Unbalanced transaction",
		Lines: []ledger.LineInput{
			{
				AccountID: debitAccountID,
				Debit:     100,
				Credit:    0,
			},
			{
				AccountID: creditAccountID,
				Debit:     0,
				Credit:    90,
			},
		},
	}

	_, _, err := svc.CreateTransaction(ctx, input)
	if err == nil {
		t.Fatalf("expected error for unbalanced transaction, got nil")
	}
	if !errors.Is(err, ledger.ErrUnbalancedTransaction) {
		t.Fatalf("expected ErrUnbalancedTransaction, got %v", err)
	}
}

func TestCreateTransaction_TooFewLinesFails(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	userID, debitAccountID, _ := setupTestAccounts(t, db)

	svc := &ledger.Service{DB: db}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := ledger.CreateTransactionInput{
		UserID:      userID,
		Source:      "manual",
		Description: "Too few lines",
		Lines: []ledger.LineInput{
			{
				AccountID: debitAccountID,
				Debit:     100,
				Credit:    0,
			},
		},
	}

	_, _, err := svc.CreateTransaction(ctx, input)
	if err == nil {
		t.Fatalf("expected error for too few lines, got nil")
	}
	if !errors.Is(err, ledger.ErrInvalidLines) {
		t.Fatalf("expected ErrInvalidLines, got %v", err)
	}
}


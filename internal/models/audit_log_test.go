package models_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/your-org/ledger-engine/internal/ledger"
	"github.com/your-org/ledger-engine/internal/models"
	"github.com/your-org/ledger-engine/internal/testutil"
)

func TestAuditLogsImmutableAndRecorded(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Insert(ctx, "audit@example.com", "active")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	accountModel := &models.AccountModel{DB: db}
	debit, err := accountModel.Insert(ctx, user.ID, "Cash", models.AccountTypeAsset)
	if err != nil {
		t.Fatalf("insert debit account: %v", err)
	}
	credit, err := accountModel.Insert(ctx, user.ID, "Revenue", models.AccountTypeRevenue)
	if err != nil {
		t.Fatalf("insert credit account: %v", err)
	}

	service := &ledger.Service{DB: db}
	if _, _, err := service.CreateTransaction(ctx, ledger.CreateTransactionInput{
		UserID: user.ID,
		Source: "test",
		Lines: []ledger.LineInput{
			{AccountID: debit.ID, Debit: 250},
			{AccountID: credit.ID, Credit: 250},
		},
	}); err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	auditModel := &models.AuditLogModel{DB: db}
	logs, err := auditModel.ListByUser(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("ListByUser error: %v", err)
	}
	if len(logs) < 3 {
		t.Fatalf("expected at least 3 audit log rows, got %d", len(logs))
	}

	_, err = db.ExecContext(ctx, `UPDATE audit_logs SET action = 'tampered'`)
	if err == nil {
		t.Fatalf("expected immutability update error, got nil")
	}
	if !strings.Contains(err.Error(), "audit_logs are immutable") {
		t.Fatalf("expected immutable audit log error, got %v", err)
	}
}

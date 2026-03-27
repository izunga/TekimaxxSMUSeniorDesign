package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/models"
)

type LineInput struct {
	AccountID uuid.UUID `json:"account_id"`
	Debit     int64     `json:"debit"`
	Credit    int64     `json:"credit"`
}

type CreateTransactionInput struct {
	UserID            uuid.UUID   `json:"user_id"`
	Source            string      `json:"source"`
	ExternalReference string      `json:"external_reference,omitempty"`
	Description       string      `json:"description,omitempty"`
	Lines             []LineInput `json:"lines"`
}

type Service struct {
	DB *sql.DB
}

var (
	ErrUnbalancedTransaction = errors.New("transaction debits and credits do not balance")
	ErrInvalidLines          = errors.New("transaction must have at least two valid lines")
	ErrUnauthorizedAccount   = errors.New("one or more accounts are not owned by user")
)

// CreateTransaction creates a balanced double-entry transaction and its journal entries
// inside a single database transaction.
func (s *Service) CreateTransaction(ctx context.Context, input CreateTransactionInput) (*models.Transaction, []models.JournalEntry, error) {
	if len(input.Lines) < 2 {
		return nil, nil, ErrInvalidLines
	}

	var totalDebit, totalCredit int64
	for _, l := range input.Lines {
		if l.Debit < 0 || l.Credit < 0 {
			return nil, nil, fmt.Errorf("negative amounts not allowed")
		}
		if l.Debit == 0 && l.Credit == 0 {
			return nil, nil, fmt.Errorf("line cannot have both debit and credit equal to zero")
		}
		totalDebit += l.Debit
		totalCredit += l.Credit
	}

	if totalDebit != totalCredit {
		return nil, nil, ErrUnbalancedTransaction
	}

	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txn := &models.Transaction{
		ID:                uuid.New(),
		UserID:            input.UserID,
		Source:            input.Source,
		ExternalReference: input.ExternalReference,
		Description:       input.Description,
	}

	txnModel := &models.TransactionModel{DB: s.DB}
	if err = txnModel.InsertTx(ctx, tx, txn); err != nil {
		return nil, nil, err
	}

	accountModel := &models.AccountModel{DB: s.DB}
	for _, l := range input.Lines {
		belongs, err := accountModel.ExistsForUserTx(ctx, tx, l.AccountID, input.UserID)
		if err != nil {
			return nil, nil, err
		}
		if !belongs {
			return nil, nil, ErrUnauthorizedAccount
		}
	}

	journalModel := &models.JournalEntryModel{DB: s.DB}
	entries := make([]models.JournalEntry, 0, len(input.Lines))

	for _, l := range input.Lines {
		entry := models.JournalEntry{
			ID:            uuid.New(),
			TransactionID: txn.ID,
			AccountID:     l.AccountID,
			DebitAmount:   l.Debit,
			CreditAmount:  l.Credit,
		}
		if err = journalModel.InsertTx(ctx, tx, &entry); err != nil {
			return nil, nil, err
		}
		entries = append(entries, entry)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}

	return txn, entries, nil
}

// GetAccountBalance computes the balance for a single account from journal_entries.
func (s *Service) GetAccountBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
		SELECT COALESCE(SUM(debit_amount - credit_amount), 0)
		FROM journal_entries
		WHERE account_id = $1
	`

	var balance int64
	if err := tx.QueryRowContext(ctx, q, accountID).Scan(&balance); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return balance, nil
}

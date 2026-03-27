package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type JournalEntry struct {
	ID            uuid.UUID `json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	AccountID     uuid.UUID `json:"account_id"`
	DebitAmount   int64     `json:"debit_amount"`
	CreditAmount  int64     `json:"credit_amount"`
	CreatedAt     time.Time `json:"created_at"`
}

type JournalEntryModel struct {
	DB *sql.DB
}

func (m *JournalEntryModel) InsertTx(ctx context.Context, tx *sql.Tx, e *JournalEntry) error {
	const q = `
		INSERT INTO journal_entries (id, transaction_id, account_id, debit_amount, credit_amount)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	return tx.QueryRowContext(ctx, q,
		e.ID,
		e.TransactionID,
		e.AccountID,
		e.DebitAmount,
		e.CreditAmount,
	).Scan(&e.CreatedAt)
}


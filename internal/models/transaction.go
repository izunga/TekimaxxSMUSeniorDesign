package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	Source            string    `json:"source"`
	ExternalReference string    `json:"external_reference,omitempty"`
	Description       string    `json:"description,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type TransactionModel struct {
	DB *sql.DB
}

func (m *TransactionModel) InsertTx(ctx context.Context, tx *sql.Tx, t *Transaction) error {
	const q = `
		INSERT INTO transactions (id, user_id, source, external_reference, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	return tx.QueryRowContext(ctx, q,
		t.ID,
		t.UserID,
		t.Source,
		t.ExternalReference,
		t.Description,
	).Scan(&t.CreatedAt)
}


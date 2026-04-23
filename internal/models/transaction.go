package models

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (m *TransactionModel) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	const q = `
		SELECT id, user_id, source, external_reference, description, created_at
		FROM transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := m.DB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]Transaction, 0, limit)
	for rows.Next() {
		var txn Transaction
		if err := rows.Scan(
			&txn.ID,
			&txn.UserID,
			&txn.Source,
			&txn.ExternalReference,
			&txn.Description,
			&txn.CreatedAt,
		); err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}

	return transactions, rows.Err()
}

func NewTransactionAuditMetadata(lines int, source string) json.RawMessage {
	metadata, _ := json.Marshal(map[string]any{
		"lines":  lines,
		"source": source,
	})
	return metadata
}

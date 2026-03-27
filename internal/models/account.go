package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeRevenue   AccountType = "revenue"
	AccountTypeExpense   AccountType = "expense"
	AccountTypeEquity    AccountType = "equity"
)

type Account struct {
	ID        uuid.UUID   `json:"id"`
	UserID    uuid.UUID   `json:"user_id"`
	Name      string      `json:"name"`
	Type      AccountType `json:"type"`
	CreatedAt time.Time   `json:"created_at"`
}

type AccountModel struct {
	DB *sql.DB
}

func (m *AccountModel) Insert(ctx context.Context, userID uuid.UUID, name string, t AccountType) (*Account, error) {
	id := uuid.New()

	tx, err := m.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const q = `
		INSERT INTO accounts (id, user_id, name, type)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`

	var createdAt time.Time
	if err = tx.QueryRowContext(ctx, q, id, userID, name, string(t)).Scan(&createdAt); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &Account{
		ID:        id,
		UserID:    userID,
		Name:      name,
		Type:      t,
		CreatedAt: createdAt,
	}, nil
}

func (m *AccountModel) GetByID(ctx context.Context, id uuid.UUID) (*Account, error) {
	tx, err := m.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
		SELECT id, user_id, name, type, created_at
		FROM accounts
		WHERE id = $1
	`

	var a Account
	var t string
	if err := tx.QueryRowContext(ctx, q, id).Scan(
		&a.ID,
		&a.UserID,
		&a.Name,
		&t,
		&a.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	a.Type = AccountType(t)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &a, nil
}

func (m *AccountModel) ExistsForUserTx(ctx context.Context, tx *sql.Tx, accountID, userID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1
			FROM accounts
			WHERE id = $1 AND user_id = $2
		)
	`

	var exists bool
	if err := tx.QueryRowContext(ctx, q, accountID, userID).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

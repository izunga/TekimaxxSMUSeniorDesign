package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type UserModel struct {
	DB *sql.DB
}

func (m *UserModel) Insert(ctx context.Context, email, status string) (*User, error) {
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
		INSERT INTO users (id, email, status)
		VALUES ($1, $2, $3)
		RETURNING created_at
	`

	var createdAt time.Time
	if err = tx.QueryRowContext(ctx, q, id, email, status).Scan(&createdAt); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return &User{
		ID:        id,
		Email:     email,
		Status:    status,
		CreatedAt: createdAt,
	}, nil
}

func (m *UserModel) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	tx, err := m.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
		SELECT id, email, status, created_at
		FROM users
		WHERE id = $1
	`

	var u User
	if err := tx.QueryRowContext(ctx, q, id).Scan(
		&u.ID,
		&u.Email,
		&u.Status,
		&u.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &u, nil
}

func (m *UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	tx, err := m.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const q = `
		SELECT id, email, status, created_at
		FROM users
		WHERE email = $1
	`

	var u User
	if err := tx.QueryRowContext(ctx, q, email).Scan(
		&u.ID,
		&u.Email,
		&u.Status,
		&u.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &u, nil
}

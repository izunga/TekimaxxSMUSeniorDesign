package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/security"
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
		INSERT INTO users (id, email, email_encrypted, email_hash, email_key_id, status)
		VALUES ($1, NULL, $2, $3, $4, $5)
		RETURNING created_at
	`

	sealedEmail, emailHash, keyID, err := security.DefaultPIIProtector().SealEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	var createdAt time.Time
	if err = tx.QueryRowContext(ctx, q, id, sealedEmail, emailHash, keyID, status).Scan(&createdAt); err != nil {
		return nil, err
	}

	auditModel := &AuditLogModel{DB: m.DB}
	if err = auditModel.InsertTx(ctx, tx, &AuditLog{
		UserID:       &id,
		ActorEmail:   email,
		Action:       "user.created",
		ResourceType: "user",
		ResourceID:   id.String(),
	}); err != nil {
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

func (m *UserModel) UpsertServiceUser(ctx context.Context, email, status string) (*User, error) {
	id := uuid.New()

	sealedEmail, emailHash, keyID, err := security.DefaultPIIProtector().SealEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	const q = `
		INSERT INTO users (id, email, email_encrypted, email_hash, email_key_id, status)
		VALUES ($1, NULL, $2, $3, $4, $5)
		ON CONFLICT (email_hash) WHERE email_hash IS NOT NULL
		DO UPDATE SET
		    email = NULL,
		    email_encrypted = EXCLUDED.email_encrypted,
		    email_key_id = EXCLUDED.email_key_id,
		    status = EXCLUDED.status
		RETURNING id, status, created_at
	`

	var u User
	if err := m.DB.QueryRowContext(ctx, q, id, sealedEmail, emailHash, keyID, status).Scan(
		&u.ID,
		&u.Status,
		&u.CreatedAt,
	); err != nil {
		return nil, err
	}
	u.Email = email

	return &u, nil
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
		SELECT id, email, email_encrypted, status, created_at
		FROM users
		WHERE id = $1
	`

	var u User
	var legacyEmail sql.NullString
	var encryptedEmail sql.NullString
	if err := tx.QueryRowContext(ctx, q, id).Scan(
		&u.ID,
		&legacyEmail,
		&encryptedEmail,
		&u.Status,
		&u.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	u.Email, err = resolveStoredEmail(ctx, legacyEmail, encryptedEmail)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if legacyEmail.Valid && (!encryptedEmail.Valid || encryptedEmail.String == "") {
		_ = m.backfillEncryptedEmail(ctx, u.ID, legacyEmail.String)
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
		SELECT id, email, email_encrypted, status, created_at
		FROM users
		WHERE email_hash = $1 OR lower(email) = lower($2)
		ORDER BY CASE WHEN email_hash = $1 THEN 0 ELSE 1 END
		LIMIT 1
	`

	var u User
	var legacyEmail sql.NullString
	var encryptedEmail sql.NullString
	if err := tx.QueryRowContext(ctx, q, security.HashEmail(email), email).Scan(
		&u.ID,
		&legacyEmail,
		&encryptedEmail,
		&u.Status,
		&u.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	u.Email, err = resolveStoredEmail(ctx, legacyEmail, encryptedEmail)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if legacyEmail.Valid && (!encryptedEmail.Valid || encryptedEmail.String == "") {
		_ = m.backfillEncryptedEmail(ctx, u.ID, legacyEmail.String)
	}

	return &u, nil
}

func resolveStoredEmail(ctx context.Context, legacyEmail sql.NullString, encryptedEmail sql.NullString) (string, error) {
	if encryptedEmail.Valid && encryptedEmail.String != "" {
		return security.DefaultPIIProtector().OpenEmail(ctx, encryptedEmail.String)
	}
	if legacyEmail.Valid {
		return legacyEmail.String, nil
	}
	return "", nil
}

func (m *UserModel) backfillEncryptedEmail(ctx context.Context, id uuid.UUID, email string) error {
	sealedEmail, emailHash, keyID, err := security.DefaultPIIProtector().SealEmail(ctx, email)
	if err != nil {
		return err
	}

	_, err = m.DB.ExecContext(ctx, `
		UPDATE users
		SET email = NULL,
		    email_encrypted = $1,
		    email_hash = $2,
		    email_key_id = $3
		WHERE id = $4
		  AND (email_encrypted IS NULL OR email_encrypted = '')
	`, sealedEmail, emailHash, keyID, id)
	return err
}

package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID       `json:"id"`
	UserID       *uuid.UUID      `json:"user_id,omitempty"`
	ActorEmail   string          `json:"actor_email,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
}

type AuditLogModel struct {
	DB *sql.DB
}

func (m *AuditLogModel) InsertTx(ctx context.Context, tx *sql.Tx, logEntry *AuditLog) error {
	if logEntry.ID == uuid.Nil {
		logEntry.ID = uuid.New()
	}
	if len(logEntry.Metadata) == 0 {
		logEntry.Metadata = json.RawMessage(`{}`)
	}

	const q = `
		INSERT INTO audit_logs (id, user_id, actor_email, action, resource_type, resource_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`

	return tx.QueryRowContext(
		ctx,
		q,
		logEntry.ID,
		logEntry.UserID,
		logEntry.ActorEmail,
		logEntry.Action,
		logEntry.ResourceType,
		logEntry.ResourceID,
		[]byte(logEntry.Metadata),
	).Scan(&logEntry.CreatedAt)
}

func (m *AuditLogModel) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	const q = `
		SELECT id, user_id, actor_email, action, resource_type, resource_id, metadata, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := m.DB.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]AuditLog, 0, limit)
	for rows.Next() {
		var entry AuditLog
		if err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.ActorEmail,
			&entry.Action,
			&entry.ResourceType,
			&entry.ResourceID,
			&entry.Metadata,
			&entry.CreatedAt,
		); err != nil {
			return nil, err
		}
		logs = append(logs, entry)
	}

	return logs, rows.Err()
}

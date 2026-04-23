package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	*sql.DB
}

// New creates a new database connection pool using the DATABASE_URL
// environment variable. It pings the database before returning.
func New(ctx context.Context) (*DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	// Reasonable pool defaults; adjust as needed for production.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	deadline := time.Now().Add(60 * time.Second)
	attempt := 1
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			break
		}

		if ctx.Err() != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", ctx.Err())
		}

		if time.Now().After(deadline) {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres after %d attempts: %w", attempt, err)
		}

		log.Printf("database not ready (attempt %d): %v; retrying in 2s", attempt, simplifyDBError(err))
		attempt++

		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	wrapped := &DB{DB: db}
	if err := wrapped.ApplyMigrations(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return wrapped, nil
}

func (db *DB) ApplyMigrations(ctx context.Context) error {
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if strings.TrimSpace(migrationsDir) == "" {
		migrationsDir = "/app/migrations"
	}

	matches, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(matches)

	if len(matches) == 0 {
		return nil
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	for _, path := range matches {
		filename := filepath.Base(path)
		var alreadyApplied bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, filename).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if alreadyApplied {
			continue
		}
		if legacyApplied, err := db.legacyMigrationAlreadyApplied(ctx, filename); err != nil {
			return fmt.Errorf("check legacy migration %s: %w", filename, err)
		} else if legacyApplied {
			if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT (filename) DO NOTHING`, filename); err != nil {
				return fmt.Errorf("record legacy migration %s: %w", filename, err)
			}
			log.Printf("marked legacy migration %s as applied", filename)
			continue
		}

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", filename, err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, filename); err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		log.Printf("applied migration %s", filename)
	}

	return nil
}

func (db *DB) legacyMigrationAlreadyApplied(ctx context.Context, filename string) (bool, error) {
	switch filename {
	case "001_init.sql":
		return db.tableExists(ctx, "users")
	case "002_security_and_audit.sql":
		hasAudit, err := db.tableExists(ctx, "audit_logs")
		if err != nil {
			return false, err
		}
		hasColumn, err := db.columnExists(ctx, "users", "email_encrypted")
		if err != nil {
			return false, err
		}
		return hasAudit && hasColumn, nil
	default:
		return false, nil
	}
}

func (db *DB) tableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func (db *DB) columnExists(ctx context.Context, tableName string, columnName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	return exists, err
}

func simplifyDBError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, needle := range []string{
		"the database system is starting up",
		"connection refused",
		"failed SASL auth",
		"server refused TLS connection",
		"no such host",
	} {
		if strings.Contains(msg, needle) {
			return needle
		}
	}

	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	return msg
}

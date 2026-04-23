package testutil

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// NewTestDB opens a real PostgreSQL connection for tests and runs the initial migration.
// It uses TEST_DATABASE_URL if set, otherwise falls back to DATABASE_URL.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL must be set for DB tests")
	}

	testDSN, cleanup := createIsolatedTestSchema(t, dsn)
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		cleanup()
	})

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping postgres: %v", err)
	}

	runMigrations(t, db)

	return db
}

func createIsolatedTestSchema(t *testing.T, dsn string) (string, func()) {
	t.Helper()

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}

	adminDB, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	schemaName := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := adminDB.Exec(`CREATE SCHEMA ` + schemaName); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create test schema: %v", err)
	}

	query := parsed.Query()
	query.Set("search_path", schemaName+",public")
	parsed.RawQuery = query.Encode()

	cleanup := func() {
		_, _ = adminDB.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`)
		_ = adminDB.Close()
	}

	return parsed.String(), cleanup
}

func runMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine caller")
	}

	// Project root is three levels up: internal/testutil/db.go -> internal -> project root.
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	matches, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	sort.Strings(matches)

	for _, migrationPath := range matches {
		sqlBytes, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(migrationPath), err)
		}
		if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(migrationPath), err)
		}
	}
}

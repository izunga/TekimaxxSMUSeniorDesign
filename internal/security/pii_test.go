package security_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/your-org/ledger-engine/internal/models"
	"github.com/your-org/ledger-engine/internal/security"
	"github.com/your-org/ledger-engine/internal/testutil"
)

func TestPIIProtector_RoundTrip(t *testing.T) {
	security.ResetForTests()
	t.Setenv("SESSION_COOKIE_SECRET", "supersecurestringthatisatleast32characterslong")
	t.Setenv("KMS_MASTER_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("KMS_KEY_ID", "env-kms-v1")

	protector := security.DefaultPIIProtector()
	email := "crypto@example.com"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sealed, hash, keyID, err := protector.SealEmail(ctx, email)
	if err != nil {
		t.Fatalf("SealEmail error: %v", err)
	}
	if sealed == "" || hash == "" || keyID == "" {
		t.Fatalf("expected non-empty sealed values")
	}

	opened, err := protector.OpenEmail(ctx, sealed)
	if err != nil {
		t.Fatalf("OpenEmail error: %v", err)
	}
	if opened != email {
		t.Fatalf("expected %q, got %q", email, opened)
	}
}

func TestUserModel_StoresEncryptedEmailAndSupportsLookup(t *testing.T) {
	security.ResetForTests()
	t.Setenv("SESSION_COOKIE_SECRET", "supersecurestringthatisatleast32characterslong")
	t.Setenv("KMS_MASTER_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("KMS_KEY_ID", "env-kms-v1")

	db := testutil.NewTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	userModel := &models.UserModel{DB: db}
	user, err := userModel.Insert(ctx, "encrypted-user@example.com", "active")
	if err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	var legacyEmail sql.NullString
	var encryptedEmail sql.NullString
	var emailHash sql.NullString
	var emailKeyID sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT email, email_encrypted, email_hash, email_key_id
		FROM users
		WHERE id = $1
	`, user.ID).Scan(&legacyEmail, &encryptedEmail, &emailHash, &emailKeyID); err != nil {
		t.Fatalf("query stored user: %v", err)
	}

	if legacyEmail.Valid && legacyEmail.String != "" {
		t.Fatalf("expected plaintext email to be cleared")
	}
	if !encryptedEmail.Valid || encryptedEmail.String == "" {
		t.Fatalf("expected encrypted email to be stored")
	}
	if !emailHash.Valid || emailHash.String == "" {
		t.Fatalf("expected email hash to be stored")
	}
	if !emailKeyID.Valid || emailKeyID.String == "" {
		t.Fatalf("expected email key ID to be stored")
	}

	loaded, err := userModel.GetByEmail(ctx, "encrypted-user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail error: %v", err)
	}
	if loaded.Email != "encrypted-user@example.com" {
		t.Fatalf("expected decrypted email, got %q", loaded.Email)
	}
}

func TestUserModel_BackfillsLegacyPlaintextEmail(t *testing.T) {
	security.ResetForTests()
	t.Setenv("SESSION_COOKIE_SECRET", "supersecurestringthatisatleast32characterslong")
	t.Setenv("KMS_MASTER_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("KMS_KEY_ID", "env-kms-v1")

	db := testutil.NewTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id := uuid.New()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO users (id, email, status)
		VALUES ($1, $2, 'active')
	`, id, "legacy-user@example.com"); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	userModel := &models.UserModel{DB: db}
	loaded, err := userModel.GetByEmail(ctx, "legacy-user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail error: %v", err)
	}
	if loaded.Email != "legacy-user@example.com" {
		t.Fatalf("expected decrypted legacy email, got %q", loaded.Email)
	}

	var legacyEmail sql.NullString
	var encryptedEmail sql.NullString
	if err := db.QueryRowContext(ctx, `
		SELECT email, email_encrypted
		FROM users
		WHERE id = $1
	`, id).Scan(&legacyEmail, &encryptedEmail); err != nil {
		t.Fatalf("reload legacy user: %v", err)
	}
	if legacyEmail.Valid && legacyEmail.String != "" {
		t.Fatalf("expected legacy plaintext email to be cleared after backfill")
	}
	if !encryptedEmail.Valid || encryptedEmail.String == "" {
		t.Fatalf("expected encrypted email after backfill")
	}
}

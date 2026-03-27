-- 001_init.sql
-- Initial schema for ledger-engine (double-entry accounting)

BEGIN;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enumerations
CREATE TYPE account_type AS ENUM ('asset', 'liability', 'revenue', 'expense', 'equity');

-- Core tables

CREATE TABLE users (
    id          UUID PRIMARY KEY,
    email       TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    type        account_type NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_accounts_user_id ON accounts(user_id);

CREATE TABLE transactions (
    id                  UUID PRIMARY KEY,
    user_id             UUID NOT NULL REFERENCES users(id),
    source              TEXT NOT NULL,
    external_reference  TEXT,
    description         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);

CREATE TABLE journal_entries (
    id              UUID PRIMARY KEY,
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    account_id      UUID NOT NULL REFERENCES accounts(id),
    debit_amount    BIGINT NOT NULL DEFAULT 0,
    credit_amount   BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_non_negative_amounts
        CHECK (debit_amount >= 0 AND credit_amount >= 0),

    CONSTRAINT chk_non_zero_line
        CHECK (debit_amount <> 0 OR credit_amount <> 0)
);

CREATE INDEX idx_journal_entries_transaction_id ON journal_entries(transaction_id);
CREATE INDEX idx_journal_entries_account_id ON journal_entries(account_id);

-- Immutable journal_entries: prevent UPDATE and DELETE

CREATE OR REPLACE FUNCTION prevent_update_delete()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'journal_entries are immutable';
END;
$$;

CREATE TRIGGER journal_no_update
BEFORE UPDATE ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_update_delete();

CREATE TRIGGER journal_no_delete
BEFORE DELETE ON journal_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_update_delete();

-- Enforce transaction-level balancing and minimum lines at the database level.
-- This is a DEFERRABLE constraint so it is checked at COMMIT time, after all
-- journal_entries for a given transaction have been inserted.

CREATE OR REPLACE FUNCTION ensure_transaction_balanced()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    total_debits  BIGINT;
    total_credits BIGINT;
    line_count    INTEGER;
BEGIN
    SELECT
        COALESCE(SUM(debit_amount), 0),
        COALESCE(SUM(credit_amount), 0),
        COUNT(*)
    INTO total_debits, total_credits, line_count
    FROM journal_entries
    WHERE transaction_id = NEW.transaction_id;

    IF line_count < 2 THEN
        RAISE EXCEPTION 'transaction % must have at least two journal entries', NEW.transaction_id;
    END IF;

    IF total_debits <> total_credits THEN
        RAISE EXCEPTION 'transaction % is not balanced: debits % != credits %',
            NEW.transaction_id, total_debits, total_credits;
    END IF;

    RETURN NEW;
END;
$$;

CREATE CONSTRAINT TRIGGER ensure_transaction_balanced_trigger
AFTER INSERT ON journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION ensure_transaction_balanced();

COMMIT;


# Security Model

## Identity

- Browser/API auth uses WorkOS when configured.
- Internal service-to-service auth uses `INTERNAL_SERVICE_TOKEN`.
- Ledger authorization is ownership-based for user-scoped resources.

## Protected Data

- user email is protected PII
- stored fields:
  - `email_encrypted`
  - `email_hash`
  - `email_key_id`
- plaintext email is not persisted for newly written users

## Encryption Design

- per-record random data key
- AES-256-GCM for the data payload
- wrapped data key via KMS provider abstraction
- hash-based lookup for email matching
- legacy plaintext rows are backfilled to encrypted form on access

## KMS Model

- `internal/security/kms.go` defines the provider boundary
- current provider is env-backed and production-defensible for small deployments
- cloud KMS integration remains the next deployment-layer step

## Audit Model

- `journal_entries` are immutable
- `audit_logs` are immutable via database trigger
- user, account, and transaction mutations write audit records

## Webhook Security

- Stripe signature verification runs when configured
- explicit degraded mode returns `503` when Stripe is disabled/unconfigured
- duplicate event IDs are suppressed before processing

## Remaining Security Limits

- no mTLS between services
- no cloud KMS plugin committed yet
- no automated secret rotation in repo
- one Node dependency vulnerability still requires remediation

# Tekimaxx SMU Senior Design

Multi-service backend for the Tekimax financial intelligence platform.

## Services

- `ledger-engine` (Go): user ledger, accounts, transactions, immutable journal entries, GraphQL adapter, audit logs, and PII encryption
- `webhook-handler` (Node/TypeScript): Stripe webhook ingestion and normalization
- `forecast-service` (Python/FastAPI): deterministic forecasting and what-if analysis
- `llm-service` (Python/FastAPI): Granite-powered insights and Gamma Router orchestration
- `rust-crypto` (Rust): HMAC signing and verification utility for security workflows

## What changed in this completion pass

- added internal service token support for cross-service authentication
- added Go API rate limiting
- added a bootstrap route and helper script for demo ledger accounts
- added durable file-backed Stripe event storage for the webhook service
- hardened container runtime defaults
- added envelope-encrypted user PII with a local KMS abstraction
- added immutable audit logs for core ledger mutations
- added a minimal GraphQL endpoint for ledger operations
- added planning, traceability, AI usage, and security scan docs

## Quick Start

1. Copy the environment template:

```bash
cp .env.example .env
```

If you have an older local Postgres volume from previous project iterations, reset it before the first fresh boot:

```bash
docker compose down -v
```

2. Set at least these values in `.env`:

```env
DATABASE_URL=postgresql://user:password@postgres:5432/ledger?sslmode=disable
SESSION_COOKIE_SECRET=replace-this-with-a-long-random-secret
KMS_KEY_ID=env-kms-v1
KMS_MASTER_KEY_B64=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=
INTERNAL_SERVICE_TOKEN=replace-this-with-a-long-random-internal-token
LEDGER_SERVICE_TOKEN=replace-this-with-the-same-value-as-INTERNAL_SERVICE_TOKEN
```

3. Start the stack:

```bash
docker compose build
docker compose up -d
```

4. Bootstrap the demo ledger accounts:

```bash
set -a && source .env && set +a
./scripts/bootstrap_demo.sh
```

5. Copy the printed `LEDGER_*_ACCOUNT_ID` exports into your shell or `.env`, then restart the webhook service:

```bash
docker compose up -d webhook-handler
```

## Core Endpoints

### Ledger engine

- `GET /healthz`
- `GET /graphql/schema`
- `POST /graphql`
- `GET /auth/login`
- `GET /auth/callback`
- `GET /auth/status`
- `POST /auth/logout`
- `GET /auth/me`
- `POST /accounts`
- `GET /accounts/{id}/balance`
- `POST /transactions`
- `POST /bootstrap/demo`

### Webhook handler

- `GET /health`
- `POST /stripe/webhook`
- dashboard routes from `webhook-handler/src/controllers/dashboard.controller.ts`

### Forecast service

- `GET /health`
- `POST /forecast`
- `POST /what-if`

### LLM service

- `GET /health`
- `POST /insights`
- `POST /analyze`

## Auth Model

- Browser/API auth into the Go ledger uses WorkOS when configured.
- Internal service-to-service calls can use `Authorization: Bearer $INTERNAL_SERVICE_TOKEN` for the Go service.
- Python services accept `X-Internal-Service-Token` when `ALLOW_INSECURE_USER_HEADER=false`.
- For local demo mode, `ALLOW_INSECURE_USER_HEADER=true` permits the existing `X-Tekimax-User-Id` flow.

## Security and Project Docs

- [RAID.md](RAID.md)
- [REQUIREMENTS_TRACEABILITY.md](REQUIREMENTS_TRACEABILITY.md)
- [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md)
- [AI_USAGE_LOG.md](AI_USAGE_LOG.md)
- [SECURITY_SCAN.md](SECURITY_SCAN.md)
- [DEMO_RUNBOOK.md](DEMO_RUNBOOK.md)
- [TEST_REPORT.md](TEST_REPORT.md)
- [notebooks/forecast_demo.ipynb](notebooks/forecast_demo.ipynb)

## Notes

- `WORKOS_CLIENT_ID` should start with `client_`.
- `webhook-handler` now persists event history to a mounted JSON file by default.
- `rust-crypto` is now a small utility for HMAC signing and verification, not a placeholder.
- User email is stored with envelope encryption and a keyed-hash lookup path.
- The Go service applies SQL migrations on startup for more reliable demo restarts.

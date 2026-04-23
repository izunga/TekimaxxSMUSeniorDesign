# Demo Runbook

This document explains how to start, validate, and demo the Tekimax microservices platform.

## Scope

Covered services:

- `postgres`
- `ledger-engine`
- `forecast-service`
- `llm-service`
- `webhook-handler`
- `rust-crypto`

Additional demo assets:

- `GET /graphql/schema` and `POST /graphql` on `ledger-engine`
- `notebooks/forecast_demo.ipynb`

## Environment

The project reads `.env` automatically through Docker Compose and service-specific config.

Note:

- Docker Compose injects variables from the repo `.env` into the containers
- `ledger-engine` may log that a local `.env` file was not found inside the container; that is harmless for the demo because Compose has already provided the environment

Minimum demo-safe values:

```env
STRIPE_SECRET_KEY=sk_test_dummy
STRIPE_WEBHOOK_SECRET=whsec_dummy
SESSION_COOKIE_SECRET=supersecurestringthatisatleast32characterslong
KMS_KEY_ID=env-kms-v1
KMS_MASTER_KEY_B64=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=
```

The checked-in `.env` and `.env.example` already include these demo defaults.

## Fresh Start

If you have old local database state from previous iterations, reset volumes first:

```bash
docker compose down -v
```

## Start The Platform

Build and start everything:

```bash
docker compose up --build
```

Start in the background:

```bash
docker compose up --build -d
```

## Why Startup Is Stable Now

- `postgres` has a real `pg_isready` healthcheck
- `ledger-engine` waits on healthy Postgres in Compose
- `ledger-engine` also retries database connection attempts for up to 60 seconds at the application level
- `ledger-engine` applies migrations at startup, including GraphQL/encryption/audit schema changes
- `webhook-handler` no longer crashes when Stripe credentials are missing
- `rust-crypto` is kept alive in Compose for demo visibility instead of exiting like a CLI

## Service Expectations

### postgres

- Should show `healthy` after startup
- Exposes port `5432`

### ledger-engine

- Should stay running after Postgres becomes healthy
- Exposes port `8080`
- Health endpoint: `GET /healthz`

### forecast-service

- Should stay running
- Internal service on port `8001`
- Health endpoint: `GET /health`

### llm-service

- Should stay running
- Internal service on port `8002`
- Health endpoint: `GET /health`

### webhook-handler

- Should stay running even if Stripe is not really configured
- Exposes port `3001`
- Health endpoint: `GET /health`
- If Stripe is not configured, `POST /stripe/webhook` returns a safe degraded response

### rust-crypto

- Exists in the stack as the crypto component
- Stays alive for demo purposes via `sleep infinity`

## Verification Commands

Check container status:

```bash
docker compose ps
```

Expected:

- `postgres` healthy
- `ledger-engine` healthy
- `forecast-service` healthy
- `llm-service` healthy
- `webhook-handler` healthy
- `rust-crypto` running

Check logs:

```bash
docker compose logs --tail=100
```

Check specific services:

```bash
docker compose logs --tail=100 postgres ledger-engine webhook-handler forecast-service llm-service rust-crypto
```

## Functional Test Paths

### 1. Ledger health

```bash
curl http://localhost:8080/healthz
```

Expected response:

```text
ok
```

### 2. Webhook handler health

```bash
curl http://localhost:3001/health
```

Expected response:

```json
{"status":"ok"}
```

### 3. GraphQL schema discovery

```bash
curl http://localhost:8080/graphql/schema
```

### 4. GraphQL me query

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Authorization: Bearer $INTERNAL_SERVICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"operationName":"Me","query":"query Me { me { id email status } }","variables":{}}'
```

### 5. Forecast service health

Run from inside the container network if needed:

```bash
docker compose exec forecast-service wget -qO- http://127.0.0.1:8001/health
```

### 6. LLM service health

```bash
docker compose exec llm-service wget -qO- http://127.0.0.1:8002/health
```

### 7. Stripe degraded mode

If Stripe env vars are unset, verify graceful behavior:

```bash
curl -X POST http://localhost:3001/stripe/webhook
```

Expected:

- no container crash
- safe error response

### 8. Demo bootstrap for ledger accounts

If using the ledger bootstrap route:

```bash
./scripts/bootstrap_demo.sh
```

This prints the generated account IDs needed by the webhook-to-ledger flow.

## Requirement Mapping

### Core runtime requirements

- Go ledger service runs cleanly
- Python forecast service runs cleanly
- Python LLM service runs cleanly
- Node webhook handler runs cleanly in normal or degraded Stripe mode
- Rust crypto service does not destabilize the stack

### Demo reliability requirements

- Docker Compose startup ordering uses service health
- Go startup includes retry logic, not just Docker orchestration
- Demo-safe env defaults prevent avoidable crashes

## Shutdown

Stop services:

```bash
docker compose down
```

Stop and remove volumes:

```bash
docker compose down -v
```

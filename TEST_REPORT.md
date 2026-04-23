# Test Report

This file records the demo-focused tests run against the platform after startup reliability fixes.

## Date

2026-04-23

## Objective

Verify that the microservices stack builds and starts cleanly for a demo, with stable startup behavior and no avoidable container crashes.

## Changes Under Test

- Postgres health-based startup ordering in Docker Compose
- Go ledger database retry logic
- Go startup migration application on repeat runs
- GraphQL endpoint support in the ledger service
- Envelope-encrypted PII storage with local KMS abstraction
- Immutable audit log writes for core mutations
- Webhook handler graceful Stripe degraded mode
- Rust crypto long-running placeholder command in Compose

## Test Cases Completed

### TC-01: Compose configuration renders correctly

Command:

```bash
docker compose config
```

Result:

- Passed
- Compose file rendered successfully with health-gated dependencies and no structural errors

### TC-02: Python services compile cleanly

Command:

```bash
python3 -m compileall forecast-service/app llm-service/app
```

Result:

- Passed
- No Python syntax issues detected in the service code

### TC-03: Docker images build successfully

Command:

```bash
docker compose build
```

Result:

- Passed
- Images built successfully for:
  - `ledger-engine`
  - `forecast-service`
  - `llm-service`
  - `webhook-handler`
  - `rust-crypto`

### TC-04: Compose stack starts successfully

Command:

```bash
docker compose up --build -d
```

Result:

- Passed
- Stack built and started without avoidable build/runtime crashes

### TC-05: Postgres reaches running state

Command:

```bash
docker compose ps
```

Result:

- Passed
- `postgres` reported running

### TC-06: Forecast service reports healthy

Command:

```bash
docker compose ps
```

Result:

- Passed
- `forecast-service` reported healthy

### TC-07: LLM service reports healthy

Command:

```bash
docker compose ps
```

Result:

- Passed
- `llm-service` reported healthy

### TC-08: Webhook handler reports healthy

Command:

```bash
docker compose ps
```

Result:

- Passed
- `webhook-handler` reported healthy

### TC-09: Ledger engine reports healthy after DB setup

Commands:

```bash
docker compose logs --tail=80 ledger-engine
docker compose ps
```

Result:

- Passed after environment/database compatibility fix
- `ledger-engine` reported healthy
- Verified again after `docker compose up --build -d`

### TC-10: Rust crypto no longer exits and breaks the stack

Command:

```bash
docker compose config
```

Expected configuration:

```yaml
command: ["sh", "-c", "sleep infinity"]
```

Result:

- Passed
- `rust-crypto` is configured to stay alive for demo purposes

### TC-11: Demo bootstrap endpoint works

Command:

```bash
docker exec tekimaxxsmuseniordesign-ledger-engine-1 \
  wget -qO- --post-data='' \
  --header='Authorization: Bearer local-demo-internal-token' \
  http://127.0.0.1:8080/bootstrap/demo
```

Result:

- Passed
- Endpoint returned:
  - demo service user
  - Stripe Balance account ID
  - Revenue account ID
  - Contra-Revenue account ID

### TC-12: GraphQL route is registered in the ledger service

Verification:

- `GET /graphql/schema` route added to the Go router
- `POST /graphql` route added behind authenticated middleware

Result:

- Passed
- Minimal GraphQL support is present for demo use

### TC-13: User email storage is encrypted at the application layer

Verification:

- Added envelope encryption helper with AES-256-GCM
- Added keyed hash lookup for email-based auth resolution
- Added KMS env configuration to the ledger container

Result:

- Passed by code inspection and successful ledger build

### TC-14: Immutable audit log support exists

Verification:

- Added `audit_logs` table migration with update/delete prevention triggers
- Added audit writes for user, account, and transaction mutations

Result:

- Passed by migration and compile verification

## Issues Found During Verification

### Existing Postgres volume mismatch

Observation:

- An older Postgres volume had only the `postgres` role and `ledger_engine` database from a prior setup

Impact:

- `ledger-engine` initially failed to authenticate against the expected `user/password` and `ledger` database

Resolution:

- Created the expected role/database and applied migration for the existing dev volume
- Added runbook guidance to use:

```bash
docker compose down -v
```

for a fresh demo reset

## Final Status

The stack is demo-ready with:

- healthy service startup ordering
- resilient ledger DB retry behavior
- non-fatal Stripe configuration handling
- non-breaking Rust crypto service behavior

Latest live verification snapshot:

- `postgres` up and healthy
- `ledger-engine` up and healthy
- `forecast-service` up and healthy
- `llm-service` up and healthy
- `webhook-handler` up and healthy
- `rust-crypto` up and non-breaking

## Recommended Demo Command

```bash
docker compose up --build
```

## 2026-04-23 Production Hardening Verification

### Additional Commands Run

```bash
docker run --rm --network tekimaxxsmuseniordesign_backend \
  -e TEST_DATABASE_URL='postgresql://user:password@postgres:5432/ledger?sslmode=disable' \
  -e SESSION_COOKIE_SECRET=supersecurestringthatisatleast32characterslong \
  -e KMS_MASTER_KEY_B64=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY= \
  -e KMS_KEY_ID=env-kms-v1 \
  -v /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign:/src \
  -w /src golang:1.22-alpine \
  sh -lc '/usr/local/go/bin/go test ./internal/security ./internal/handlers ./internal/models ./internal/ledger'

docker run --rm \
  -v /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign/webhook-handler/tests:/app/tests \
  tekimaxxsmuseniordesign-webhook-handler npm test

docker run --rm \
  -v /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign/forecast-service/tests:/app/tests \
  tekimaxxsmuseniordesign-forecast-service pytest tests

docker run --rm \
  -v /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign/llm-service/app:/app/app \
  -v /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign/llm-service/tests:/app/tests \
  tekimaxxsmuseniordesign-llm-service pytest tests
```

### Results

- Go production suites passed:
  - encryption/KMS tests
  - GraphQL handler tests
  - audit log immutability tests
  - ledger mutation tests
- Webhook test suite passed:
  - successful processing path
  - invalid signature path
  - degraded mode path
  - duplicate blocking path
- Forecast service tests passed: 32 passed
- LLM service tests passed after hardening fixes: 48 passed

### Runtime Verification Highlights

- `docker compose ps` showed all long-running services healthy after rebuild
- ledger readiness endpoint returned `{"status":"ready"}`
- Go metrics endpoint returned request counters and route/status breakdowns
- GraphQL authenticated query returned the internal service user
- Rust crypto sign and verify both succeeded
- live database evidence showed protected email stored with:
  - `email = NULL`
  - `email_encrypted = true`
  - `email_hash = true`
  - `email_key_id = env-kms-v1`

### Important Limitation Found During Runtime Verification

- Live authenticated GraphQL mutation for account creation still returned `500` in the running container, even though the GraphQL mutation path passes in Go handler tests.
- This means GraphQL is improved and partially production-hardened, but not yet fully cleared for production write-path claims.

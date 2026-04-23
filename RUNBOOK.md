# Runbook

## Normal Startup

```bash
docker compose up --build -d
docker compose ps
```

Healthy state:

- `postgres`: healthy
- `ledger-engine`: healthy
- `forecast-service`: healthy
- `llm-service`: healthy
- `webhook-handler`: healthy
- `rust-crypto`: running

## Health and Readiness

- Ledger liveness: `GET http://localhost:8080/healthz`
- Ledger readiness: `GET http://localhost:8080/readyz`
- Ledger metrics: `GET http://localhost:8080/metrics`
- Webhook liveness: `GET http://localhost:3001/health`
- Webhook readiness: `GET http://localhost:3001/ready`

## Common Checks

GraphQL schema helper:

```bash
docker exec tekimaxxsmuseniordesign-ledger-engine-1 wget -qO- http://127.0.0.1:8080/graphql/schema
```

Authenticated GraphQL query:

```bash
docker exec tekimaxxsmuseniordesign-ledger-engine-1 \
  wget -qO- \
  --header='Authorization: Bearer local-demo-internal-token' \
  --header='Content-Type: application/json' \
  --post-data='{"operationName":"Me","query":"query Me { me { id email status } }","variables":{}}' \
  http://127.0.0.1:8080/graphql
```

Rust sign flow:

```bash
docker exec tekimaxxsmuseniordesign-rust-crypto-1 /app/rust-crypto sign --secret demo-secret --message demo-message
```

## Incident Hints

- `ledger-engine` not ready:
  Check `docker compose logs ledger-engine postgres`.
- webhook degraded:
  Check `STRIPE_MODE`, `STRIPE_SECRET_KEY`, and `STRIPE_WEBHOOK_SECRET`.
- GraphQL unauthorized:
  Confirm `INTERNAL_SERVICE_TOKEN` or browser session auth is present.
- encryption errors:
  Confirm `KMS_MASTER_KEY_B64` is set and valid in production.

## Shutdown

```bash
docker compose down
```

For a full reset:

```bash
docker compose down -v
```

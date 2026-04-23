# Deployment

## Deployment Model

The repository is production-defensible for a small deployment using Docker Compose.

Why Compose remains acceptable here:

- service count is small
- startup ordering and health checks are explicit
- restart policies are configured
- operational complexity is lower than introducing Kubernetes solely for packaging

## Required Production Configuration

- `APP_ENV=production`
- strong `SESSION_COOKIE_SECRET`
- strong `INTERNAL_SERVICE_TOKEN`
- `KMS_MASTER_KEY_B64` set to a valid 32-byte AES key in base64
- `STRIPE_MODE=required` or `disabled`
- `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` when Stripe is enabled

## Deployment Steps

1. Build and start:

```bash
docker compose up --build -d
```

2. Verify readiness:

```bash
docker compose ps
docker compose logs --tail=100
```

3. Verify ledger readiness and GraphQL:

```bash
docker exec tekimaxxsmuseniordesign-ledger-engine-1 wget -qO- http://127.0.0.1:8080/readyz
docker exec tekimaxxsmuseniordesign-ledger-engine-1 wget -qO- http://127.0.0.1:8080/graphql/schema
```

## Security Scan Workflow

Use:

- `SECURITY_SCAN.md`
- `.github/workflows/ci-security.yml`

The workflow covers:

- Go tests
- Python tests
- webhook tests
- Trivy filesystem/container scan

## What Still Belongs to Environment Provisioning

- cloud KMS provider binding
- secret manager integration
- managed Postgres backups / PITR
- TLS termination and certificate rotation
- external log and metrics sink

# Production Readiness Checklist

## Completed

- [x] Reliable Compose startup ordering and restart policies
- [x] Go DB retry plus readiness checks
- [x] App-side SQL migrations for clean and existing databases
- [x] Authenticated GraphQL query and mutation support
- [x] GraphQL negative-path verification
- [x] Envelope encryption for protected PII
- [x] Hash-based lookup for protected email fields
- [x] Immutable audit log storage and immutability verification
- [x] Durable Stripe event storage and duplicate suppression
- [x] Webhook degraded mode and invalid-signature handling
- [x] Structured request logging and request IDs in Go
- [x] Metrics endpoint in Go
- [x] Docker hardening: non-root, read-only FS, tmpfs, health checks
- [x] Production docs and operator runbooks
- [x] Verification evidence captured in `TEST_REPORT.md`

## Required Before Calling It Fully Production-Ready

- [ ] Wire a real cloud KMS provider in deployment
- [ ] Resolve the remaining live GraphQL mutation `500` observed in container runtime verification
- [ ] Resolve the remaining Node vulnerability reported during `npm ci`
- [ ] Add automated Stripe replay / DLQ workflow
- [ ] Add backup, restore, and PITR automation
- [ ] Add cross-service tracing or shared telemetry pipeline
- [ ] Add broader end-to-end integration coverage across services

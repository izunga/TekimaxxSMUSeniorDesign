# Production Gap Analysis

Date: 2026-04-23

This assessment compares the current repository against the requirements document and practical expectations for a small production deployment.

## Status Summary

| Area | Status | Notes |
|---|---|---|
| API surface: REST | Complete | Core ledger, forecast, LLM, and webhook REST endpoints are implemented and verified. |
| API surface: GraphQL | Risky | Authenticated GraphQL query and tested mutation paths exist, but a live runtime mutation still returns `500` in the current container image and needs one more fix before production write-path confidence is justified. |
| Authn/authz | Partial | WorkOS/browser auth plus internal service token flow exist. Authorization checks are present for ledger ownership. Fine-grained roles are not implemented. |
| Service-to-service auth | Partial | Internal service token flow is implemented. Mutual TLS and signed request envelopes are not implemented. |
| Ledger correctness and immutability | Complete | Balanced transactions, immutable journal entries, audit logs, and transaction tests are in place. |
| Stripe ingestion durability and replay safety | Partial | Durable event storage and duplicate detection exist. Automated replay tooling and backoff queueing are not implemented. |
| Encryption of PII | Complete | User email is envelope-encrypted with AES-256-GCM, stored hashed for lookup, and verified by tests. |
| KMS / key management | Partial | Clean provider abstraction and local production-defensible provider are implemented. Cloud KMS provider wiring remains deployment work. |
| Audit logging | Complete | Immutable audit log table exists and core ledger mutations are recorded and tested. |
| Secrets handling | Partial | Startup validation and env classification exist. Dedicated secret manager integration is still external to this repo. |
| Database migrations | Complete | App-side migration runner supports clean DBs and existing DBs; tests use isolated schemas. |
| Health/readiness/liveness | Complete | Separate `healthz`, `readyz`, `/health`, and `/ready` endpoints exist where relevant. |
| Error handling and retries | Partial | DB retry and guarded webhook/LLM behaviors exist. Queue-based retries and dead-letter handling are still missing. |
| Observability: logs, metrics, tracing | Partial | Request IDs, structured logs, and a metrics endpoint exist in the Go service. Cross-service tracing is not implemented. |
| Rate limiting | Complete | Go API rate limiting is implemented and active. |
| Input validation | Partial | Core request validation exists, especially in Go and Python. GraphQL validation is bounded but not schema-driven. |
| Container hardening | Partial | Non-root, read-only filesystems, tmpfs, health checks, and restart policies are present. Resource quotas and signed images are not. |
| CVE scanning | Partial | CI workflow and security scan documentation exist. Remediation is still needed for at least one reported Node vulnerability. |
| Test coverage | Partial | Strong coverage now exists for ledger, encryption, GraphQL, webhook, forecast, and LLM behavior. End-to-end cross-service tests remain limited. |
| Deployment/runbook readiness | Complete | Runbook, deployment guide, production checklist, and test report are present. |
| Backup/recovery assumptions | Risky | Backups, PITR, and restore drills are documented as operational requirements but not automated in repo. |

## Biggest Remaining Risks

1. Cloud KMS integration is not wired yet. The interface is ready, but production deployment still needs a real external KMS provider.
2. GraphQL is operational but intentionally narrow. It is not a general-purpose GraphQL platform with full schema tooling, persisted queries, or spec-level complexity analysis.
3. Stripe replay tooling is manual. Durable persistence and idempotency exist, but there is no automated replay worker or dead-letter queue.
4. Observability is strongest in the Go service. Python and Node services still rely mostly on conventional logging rather than shared structured telemetry and tracing.
5. Backup and disaster recovery remain an operations concern, not an automated repo capability.

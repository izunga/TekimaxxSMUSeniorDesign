# Requirements Traceability

This checklist maps the requirements PDF to the current repository state and identifies what still needs to be completed.

Legend:

- `Implemented`: present in code now
- `Partial`: some implementation exists but does not fully satisfy the requirement
- `Missing`: not implemented yet

## 1. System Vision and Educational Layer

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| Financial Intelligence backend that turns raw transactions into insights | Partial | Ledger, forecast, and LLM services exist | End-to-end workflow is present, but data/auth/security wiring is incomplete |
| API responses should support an educational layer with explainers, tips, glossaries | Partial | Forecast `insights` field and LLM plain-language responses | No dedicated glossary/tips schema or consistent educational metadata across APIs |

## 2. API Layer and Core Services (Go)

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| REST endpoints for Web/Mobile UI | Implemented | `cmd/api/main.go` routes | Need final API docs and integrated demo path |
| GraphQL support | Implemented | `POST /graphql` and `GET /graphql/schema` in the Go service | Keep supported operations documented |
| Zero-trust networking for service-to-service calls via WorkOS | Partial | Go service uses WorkOS auth | Internal Python services and webhook flow do not verify WorkOS-backed identity |
| Rate limiting middleware | Implemented | `internal/middleware/ratelimit.go` wired in `cmd/api/main.go` | Add load testing if needed |
| Finance Tracker real-time ledger per user | Implemented | User-scoped accounts, transactions, journal entries | Need demo data and integration validation |
| Immutable event log / immutable journaling | Partial | `journal_entries` are immutable in SQL | Cross-service audit logs are not comprehensive |
| Mojo for high-performance paths | Missing | No Mojo code found | Clarify if required or out of scope |
| Field-level encryption for PII | Implemented | User email is envelope-encrypted with AES-256-GCM and keyed-hash lookup | Extend coverage if new PII fields are added |
| Rust modules for critical crypto ops | Implemented | `rust-crypto` signs and verifies HMAC-SHA256 payloads | Extend only if sponsor asks for more algorithms |

## 3. Stripe Pipeline Integration

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| Ingest Stripe webhooks for charges, refunds, subscriptions | Partial | Webhook handler supports charges, refunds, invoice payments | Add subscription event support if required by sponsor |
| Normalize and route to finance tracker automatically | Partial | Normalization and ledger posting exist | Durable persistence and production-safe auth are incomplete |
| No raw card data stored | Implemented | Webhook pipeline uses Stripe event references, not card storage | Keep documented and validated |
| Server-side payment API calls only | Partial | Webhook logic is server-side | Broader Stripe API usage beyond webhooks is not shown or documented |

## 4. Forecasting and AI Layer (Python)

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| Forecasting service with statistical/ML-based models | Implemented | Moving average, exponential smoothing, regression | Needs verified runtime tests and demo dataset |
| What-if scenarios | Implemented | `/what-if` endpoint exists | No notebook integration yet |
| Embedded Jupyter notebooks in UI | Partial | `notebooks/forecast_demo.ipynb` added as a demo artifact | UI embedding remains outside this backend repo |
| IBM Granite for contextual financial advice | Implemented | LLM service via Ollama Granite | Requires working Ollama demo setup |
| Guardrail module for hallucination/compliance | Implemented | Guardrails and tests exist | Expand coverage and define fallback behavior |
| Gamma Router for direct call vs LLM routing | Implemented | `llm-service/app/services/gamma_router.py` | Needs authenticated internal service calls |

## 5. Deployment and Infrastructure

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| Microservices-inspired deployment | Implemented | Compose contains multiple services | Needs hardened production posture |
| Hardened Docker images | Implemented | Non-root users, slim/alpine bases, and runtime hardening are in place | Keep scanning before releases |
| Minimal OS images | Implemented | Slim/alpine runtime images are used across services | Continue periodic review |
| Read-only filesystems | Implemented | Compose uses `read_only` and `tmpfs` where feasible | Validate new writable paths as features evolve |
| Image CVE scanning before push | Missing | No CI/CD or scanning config found | Add documented workflow or CI step |
| AES-256 encryption at rest with KMS-managed keys | Implemented | Local env-backed KMS abstraction wraps per-record data keys and encrypts user email with AES-256-GCM | Swap provider for cloud KMS in production if desired |
| Immutable audit logs for every data mutation | Implemented | `audit_logs` table is immutable and core user/account/transaction mutations are recorded | Extend to future mutating endpoints |

## 6. Project Governance Deliverables

| Requirement | Status | Evidence | Gap |
|---|---|---|---|
| RAID map before structured development | Implemented | `RAID.md` | Keep updated |
| Document AI usage with prompt log and review rationale | Missing | No AI usage log was present | Add `AI_USAGE_LOG.md` and maintain it through project completion |

## Priority Completion Checklist

### Must complete for requirements alignment

- [x] Add durable webhook event persistence in PostgreSQL or another durable store
- [x] Implement zero-trust internal authentication across all services
- [x] Add rate limiting to the Go API layer
- [x] Implement the Rust crypto module
- [x] Add field-level encryption for sensitive PII
- [x] Harden Docker runtime settings
- [x] Add immutable audit records beyond journal entries
- [x] Create and maintain `AI_USAGE_LOG.md`

### Should complete for a stronger final delivery

- [x] Add minimal GraphQL support or written scope approval for REST-only
- [x] Add notebook artifacts or documented notebook integration strategy
- [x] Add CVE scanning workflow documentation or CI configuration
- [x] Add end-to-end demo instructions with seed data

### Nice to have if time remains

- [ ] Clarify Mojo scope and add a note if intentionally not implemented
- [ ] Add replay tooling for failed Stripe events
- [ ] Add more observability around cross-service requests

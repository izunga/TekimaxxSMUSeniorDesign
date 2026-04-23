# RAID Map

This document captures the current Risks, Assumptions, Issues, and Dependencies for the Tekimax API Ecosystem described in the requirements document.

## Scope

Covered areas:

- Go ledger engine
- Stripe ingestion pipeline
- Forecasting service
- AI advisory service
- Infrastructure and deployment
- Security and compliance controls

## Risks

| ID | Area | Risk | Impact | Likelihood | Mitigation |
|---|---|---|---|---|---|
| R1 | Identity | Internal services do not share one verified auth model. The Python services currently trust `X-Tekimax-User-Id`, while the Go service uses WorkOS-backed auth. | High | High | Standardize on signed internal service tokens or JWT verification for all service-to-service calls. |
| R2 | Stripe pipeline | Stripe event idempotency is stored in memory only, so duplicates and failed events are lost on restart. | High | High | Move webhook event persistence to PostgreSQL and add replay tooling. |
| R3 | Ledger security | The webhook handler can silently skip ledger persistence if service token or account IDs are missing. | High | Medium | Fail fast in production mode when ledger persistence prerequisites are not configured. |
| R4 | Compliance | Field-level encryption for PII is not implemented. | High | Medium | Add encrypted columns or service-layer encryption for sensitive fields before storing PII. |
| R5 | Crypto | The Rust cryptography service is currently a placeholder and does not offload signing or verification. | Medium | High | Implement minimal signing and verification endpoints or library bindings in Rust and integrate them into the Go/Node services. |
| R6 | Deployment | Containers are not consistently hardened with non-root users, read-only filesystems, and related runtime controls. | Medium | Medium | Harden Dockerfiles and `docker-compose.yml`, then document the controls. |
| R7 | Forecast accuracy | Forecasting endpoints are present, but results may be misleading without production-quality seed data, validation datasets, and model selection criteria. | Medium | Medium | Add fixture datasets, baseline metrics, and a validation note in docs. |
| R8 | LLM output safety | Guardrails exist, but there is still risk of unsupported or overconfident financial advice reaching users. | High | Medium | Expand guardrail tests, add refusal/fallback behavior, and keep prompts constrained to data-grounded explanations. |
| R9 | Demo reliability | End-to-end local startup depends on several environment variables and external services such as WorkOS and Ollama. | Medium | High | Add a demo mode with mock identity/data and a single documented startup path. |
| R10 | Auditability | Immutable journal entries exist, but not every cross-service mutation currently creates a durable audit trail. | High | Medium | Add audit tables for webhook processing, internal auth events, and sensitive operations. |
| R11 | API surface | Requirements mention REST/GraphQL, but only REST endpoints are implemented. | Medium | Medium | Either implement a minimal GraphQL layer or document REST-only scope approval. |
| R12 | Performance | The requirements mention Mojo for high-performance paths, but no Mojo component exists. | Low | Medium | Treat Mojo as a stretch item unless explicitly required by the instructor or sponsor. |

## Assumptions

| ID | Assumption | Why it matters | Validation needed |
|---|---|---|---|
| A1 | PostgreSQL is the system of record for ledger and audit data. | Service design and migrations already assume Postgres. | Confirm acceptable for final project review. |
| A2 | WorkOS is only required at the platform edge, not necessarily for every internal container hop, if equivalent zero-trust token verification is used. | This affects how internal auth should be implemented. | Confirm with sponsor/instructor if “via WorkOS” must be literal for all internal calls. |
| A3 | Jupyter notebook support can be satisfied by a documented integration plan or notebook artifacts rather than a fully embedded notebook UI in this repo. | Reduces frontend dependency for the senior design milestone. | Confirm expected deliverable level. |
| A4 | IBM Granite via Ollama is acceptable as the required open-source AI advisory model path. | Current LLM service is built around this assumption. | Verify final deployment/demo environment can run Ollama. |
| A5 | “Immutable audit logs” can be interpreted as append-only database records with update/delete protection. | Determines database design scope. | Confirm this is sufficient for review. |
| A6 | A Docker Compose based local environment is acceptable for “deployment and infrastructure” demonstration. | Keeps the milestone achievable without Kubernetes. | Confirm no cloud deployment is required. |
| A7 | The current services can be assessed as a microservices-inspired backend even without a dedicated API gateway binary, if gateway concerns are implemented in Go. | Affects final architecture story. | Validate whether a separate gateway service is expected. |
| A8 | The main project deliverable is backend-focused; no full web/mobile UI is required to pass the API implementation milestone. | Prioritizes service completion over frontend work. | Confirm with course expectations. |

## Issues

| ID | Area | Current issue | Evidence | Priority |
|---|---|---|---|---|
| I1 | Go runtime | Local `go test ./...` could not run in this environment because `go` is not installed. | Local verification result | High |
| I2 | Python runtime | Local `pytest` could not collect tests because `fastapi` and other service deps are not installed in the host environment. | Local verification result | High |
| I3 | Stripe persistence | Webhook events are stored in `InMemoryEventRepository`, not Postgres. | `webhook-handler/src/repositories/event.repository.ts` | High |
| I4 | Rust crypto | `rust-crypto/src/main.rs` is still the default “Hello, world!” program. | `rust-crypto/src/main.rs` | High |
| I5 | Auth consistency | Internal Python endpoints only require `X-Tekimax-User-Id`, which is not sufficient zero-trust auth. | `forecast-service/app/router/*.py`, `llm-service/app/router/*.py` | High |
| I6 | Rate limiting | No rate limiting middleware is present in the Go API. | `cmd/api/main.go` | Medium |
| I7 | GraphQL | No GraphQL endpoint exists. | Repository inspection | Medium |
| I8 | Security hardening | Compose file does not set read-only filesystems or container security options. | `docker-compose.yml` | Medium |
| I9 | PII encryption | No field-level encryption implementation is present for sensitive data. | Repository inspection | High |
| I10 | AI governance docs | No RAID document, requirements traceability checklist, or AI usage log was present before this addition. | Repository inspection | Medium |

## Dependencies

| ID | Dependency | Type | Needed for |
|---|---|---|---|
| D1 | PostgreSQL 15 | Runtime | Ledger storage, migrations, forecast queries, durable webhook event storage |
| D2 | WorkOS credentials and callback configuration | External service | Browser auth flow and token-backed user resolution |
| D3 | Stripe secret key and webhook secret | External service | Authentic Stripe webhook verification |
| D4 | Ollama with IBM Granite model available locally | Runtime | LLM insights and advisory features |
| D5 | Docker and Docker Compose | Tooling | Consistent local startup and demo environment |
| D6 | Python service dependencies from `requirements.txt` | Tooling/runtime | Forecast and LLM services |
| D7 | Go toolchain | Tooling/runtime | Ledger build and automated tests |
| D8 | Node.js/npm | Tooling/runtime | Webhook handler build and execution |
| D9 | Security design decision on internal auth tokens | Architecture | Zero-trust service-to-service authentication |
| D10 | Sponsor/instructor clarification on GraphQL, Mojo, and notebook depth | Governance | Final scope definition |

## Recommended Next Actions

1. Normalize authentication across services and remove trust in plain `X-Tekimax-User-Id`.
2. Replace in-memory webhook event storage with PostgreSQL-backed persistence.
3. Implement the Rust crypto module and connect it to one real security use case.
4. Harden containers and document the deployment controls.
5. Add an `AI_USAGE_LOG.md` file and keep it updated as development continues.

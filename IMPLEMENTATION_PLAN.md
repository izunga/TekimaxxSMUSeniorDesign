# Phase-Based Implementation Plan

This plan is designed to move the project from the current repository state to a review-ready delivery that aligns with the main requirements document.

## Goal

Deliver a secure, demonstrable backend system with:

- a working Go ledger
- durable Stripe ingestion
- functioning forecast and LLM advisory services
- clear security controls
- documented scope, risks, and AI-assisted development process

## Phase 1: Make the System Runnable End to End

Objective:
Establish one reliable local/demo workflow that starts every required service and exercises the main user journey.

Tasks:

1. Create or refine `.env.example` for all services.
2. Document startup using Docker Compose.
3. Add a data bootstrap path:
   - create a demo user
   - create ledger accounts needed by Stripe ingestion
   - expose the account IDs for the webhook handler
4. Add a short smoke-test script or manual checklist:
   - auth check
   - account creation
   - transaction creation
   - forecast request
   - LLM insights request
   - Stripe webhook replay
5. Make demo mode explicit:
   - `USE_MOCK_DATA=true` for forecast demos when ledger data is not yet populated

Success criteria:

- All containers start from one documented command
- The main APIs respond successfully
- A reviewer can reproduce the demo without guessing environment setup

## Phase 2: Standardize Authentication and Zero-Trust Service Calls

Objective:
Remove inconsistent trust boundaries and implement one internal auth strategy.

Tasks:

1. Define the internal trust model:
   - preferred: signed internal JWT/service token verified by receiving services
   - alternate: WorkOS-issued token propagation plus claim verification
2. Replace raw `X-Tekimax-User-Id` trust in forecast and LLM services.
3. Authenticate webhook-to-ledger communication with validated service credentials.
4. Add auth middleware to Python services.
5. Add tests for:
   - missing token
   - invalid token
   - valid token
   - user claim extraction

Success criteria:

- No internal service relies on a plain user ID header without verification
- All cross-service requests are authenticated and documented

## Phase 3: Make Stripe Ingestion Durable and Reviewable

Objective:
Turn the webhook handler from demo-safe to restart-safe.

Tasks:

1. Add a Postgres-backed Stripe event repository.
2. Create a migration for webhook event storage:
   - event ID
   - type
   - raw payload or safe subset
   - status
   - received time
   - processed time
   - failure reason
3. Preserve idempotency across restarts.
4. Add replay/admin support for failed events.
5. Update dashboard views to read from durable storage if needed.

Success criteria:

- Duplicate protection survives service restarts
- Failed events can be inspected and replayed
- Stripe pipeline can be defended as production-style, not just in-memory demo logic

## Phase 4: Close the Security Requirements

Objective:
Implement the minimum credible set of security features demanded by the requirements.

Tasks:

1. Implement Rust crypto service or library:
   - signing
   - signature verification
   - optional envelope encryption helper
2. Integrate Rust crypto into one real path:
   - session signing replacement
   - internal token verification
   - payload signing for service-to-service calls
3. Add field-level encryption for sensitive PII.
4. Define the encryption boundaries:
   - in transit
   - in service
   - at rest
5. Add immutable audit logging for:
   - auth events
   - webhook processing events
   - sensitive data access or mutation

Success criteria:

- Rust is no longer a placeholder
- Sensitive fields are demonstrably protected
- Audit logs exist outside of journal entries alone

## Phase 5: Harden Deployment

Objective:
Align the container/deployment story with the infrastructure expectations in the PDF.

Tasks:

1. Harden Dockerfiles:
   - non-root users everywhere
   - minimal runtime images
   - remove unnecessary packages
2. Harden Compose settings:
   - read-only filesystems where possible
   - explicit writable volumes/tmpfs
   - security options if supported
3. Document image scanning workflow.
4. Document secrets management assumptions.
5. Add health checks consistently across all services.

Success criteria:

- Every service has a clear hardening story
- Security controls can be pointed to in code and docs

## Phase 6: Fill the Remaining Product/Requirement Gaps

Objective:
Close scope gaps that may be raised during review.

Tasks:

1. Add rate limiting middleware in the Go API layer.
2. Decide GraphQL scope:
   - implement a minimal GraphQL endpoint
   - or document sponsor/instructor approval for REST-only delivery
3. Decide notebook scope:
   - add sample Jupyter notebooks for forecasting scenarios
   - or provide a clear integration design note
4. Expand educational output:
   - glossary fields
   - explanation metadata
   - next-step tips in API responses where appropriate

Success criteria:

- The repo has a clear answer for every line item in the requirements PDF

## Phase 7: Verification and Final Documentation

Objective:
Make the project easy to evaluate.

Tasks:

1. Run and record test results for Go, Python, and Node services.
2. Add a final verification checklist to the README.
3. Keep `RAID.md` updated.
4. Add `AI_USAGE_LOG.md` with:
   - prompt summary
   - generated output summary
   - accepted/modified/rejected rationale
5. Add architecture and demo screenshots if needed.

Success criteria:

- A reviewer can see what was built, how it works, and how it was validated

## Recommended Execution Order

1. Phase 1
2. Phase 2
3. Phase 3
4. Phase 4
5. Phase 5
6. Phase 6
7. Phase 7

## Immediate Next Task Recommendation

The highest-value next coding task is:

1. Add durable Stripe event persistence in PostgreSQL.
2. Then implement authenticated internal service calls for forecast and LLM services.

Those two changes would remove the biggest architectural weaknesses in the current repo while preserving the work already completed.

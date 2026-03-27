# Ledger Engine

Backend ledger service with WorkOS AuthKit authentication, PostgreSQL storage, and tenant-safe accounting operations.

## What this project does

- Stores `users`, `accounts`, `transactions`, and immutable `journal_entries`.
- Enforces double-entry accounting rules (balanced debits/credits).
- Uses WorkOS AuthKit for login and resolves authenticated users into the local `users` table.
- Protects account and transaction operations so users can only access their own data.

## Auth flow (end-to-end)

This service now supports full login flow:

1. Client hits `GET /auth/login`.
2. Backend redirects to WorkOS authorize endpoint with state.
3. WorkOS redirects back to `GET /auth/callback` with `code` and `state`.
4. Backend exchanges `code` for an access token.
5. Backend sets signed `session` cookie (HTTP-only).
6. Protected routes authenticate from either:
   - `Authorization: Bearer <token>`, or
   - signed `session` cookie.

Useful auth endpoints:

- `GET /auth/login`
- `GET /auth/callback`
- `GET /auth/status`
- `POST /auth/logout`
- `GET /auth/me` (protected)

Local auth test UI:

- `GET /` renders a small console page with Login/Logout/Refresh status buttons.

## Required environment variables

Create a local `.env` file (already gitignored) with:

```env
DATABASE_URL=postgres://<db_user>@localhost:5432/ledger_engine?sslmode=disable
ADDR=:8080

WORKOS_CLIENT_ID=client_xxx
WORKOS_API_KEY=sk_test_xxx
WORKOS_REDIRECT_URI=http://localhost:8080/auth/callback
WORKOS_POST_LOGIN_REDIRECT=http://localhost:8080/
SESSION_COOKIE_SECRET=<long-random-secret>
```

Optional overrides:

```env
WORKOS_AUTHORIZE_URL=https://api.workos.com/user_management/authorize
WORKOS_AUTHENTICATE_URL=https://api.workos.com/user_management/authenticate
WORKOS_USERINFO_URL=https://api.workos.com/user_management/users/me
```

## Local run

```bash
set -a && source .env && set +a
go run ./cmd/api
```

Health check:

- `http://localhost:8080/healthz`

## Notes

- `WORKOS_CLIENT_ID` should be `client_...` (not `pk_...`).
- Startup logs print auth config warnings to make misconfiguration easier to spot.
- Session cookie is signed with `SESSION_COOKIE_SECRET`; use a strong secret in non-dev environments.

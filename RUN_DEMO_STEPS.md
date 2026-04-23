# RUN_DEMO_STEPS

Mac terminal steps for running the full Tekimax platform and the demo dashboard.

## 1. Prerequisites

- Docker Desktop is installed and running.
- Node.js 20+ is installed.
- npm is available with Node.js. The demo dashboard can also use `pnpm`, but npm is the safest default on a fresh Mac.
- Optional: if you prefer `pnpm`, enable it with:

```bash
corepack enable
corepack prepare pnpm@latest --activate
```

## 2. Start the backend platform

From the repository root:

```bash
cd /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign
docker compose up --build -d
```

If Docker asks for permission or reports that the Docker daemon is unavailable, open Docker Desktop and wait until it says Docker is running.

Verify containers are running:

```bash
docker compose ps
```

Expected demo services:

- `postgres`
- `ledger-engine`
- `forecast-service`
- `llm-service`
- `webhook-handler`
- `rust-crypto`

## 3. Check service health manually

```bash
curl http://localhost:8080/readyz
curl http://localhost:8001/health
curl http://localhost:8002/health
curl http://localhost:3001/health
```

The ledger service uses `/readyz` for readiness. Forecast and LLM use `/health`.

## 4. Bootstrap demo data

```bash
curl -X POST http://localhost:8080/bootstrap/demo \
  -H "Authorization: Bearer local-demo-internal-token"
```

This creates or reuses the demo user and demo ledger accounts used by the dashboard tests.

## 5. Start the Next.js demo dashboard

In a second terminal:

```bash
cd /Users/inzwi/tekimax-platform/TekimaxxSMUSeniorDesign/demo-dashboard
npm install
npm run dev
```

Open:

```bash
open http://localhost:3000
```

If port `3000` is busy:

```bash
npm run dev -- -p 3005
open http://localhost:3005
```

If you already use `pnpm`, these equivalent commands also work:

```bash
pnpm install
pnpm dev
```

## 6. Run the live demo

In the dashboard:

1. Click `Check All Services`.
2. Click `Run All Tests`.
3. Use each test card to show the input, endpoint, output, status, and logs.
4. Use the right-side logs panel as manager-readable evidence of the latest requests.

## 7. Troubleshooting

- If a service is down, run `docker compose logs <service-name>`.
- If `curl http://localhost:8001/health` or `curl http://localhost:8002/health` fails, rebuild Compose after pulling the latest changes: `docker compose up --build -d`.
- If the dashboard cannot reach services, confirm Docker containers are running and ports `8080`, `8001`, `8002`, and `3001` are available.
- If GraphQL returns an error, the dashboard will show the real backend response instead of fabricating a success.
- If forecast data is empty, bootstrap demo data first and rerun the forecast test.

## 8. Stop everything

```bash
docker compose down
```

To remove volumes and start from a clean database:

```bash
docker compose down -v
```

#!/usr/bin/env bash
set -euo pipefail

LEDGER_ENGINE_URL="${LEDGER_ENGINE_URL:-http://localhost:8080}"
INTERNAL_SERVICE_TOKEN="${INTERNAL_SERVICE_TOKEN:-}"

if [[ -z "${INTERNAL_SERVICE_TOKEN}" ]]; then
  echo "INTERNAL_SERVICE_TOKEN is required"
  exit 1
fi

response="$(
  curl -fsS \
    -X POST \
    -H "Authorization: Bearer ${INTERNAL_SERVICE_TOKEN}" \
    -H "Content-Type: application/json" \
    "${LEDGER_ENGINE_URL}/bootstrap/demo"
)"

echo "Bootstrap response:"
echo "${response}"
echo
echo "Export these values into your shell or .env file:"
python3 - <<'PY' "${response}"
import json
import sys

body = json.loads(sys.argv[1])
for key, value in body.get("exports", {}).items():
    print(f'export {key}="{value}"')
PY

# Security Scan Workflow

This project now includes a documented workflow for using the manager-recommended `speckit-security` extension alongside the repository's own verification steps.

## Source

- Docs: [speckit.tekimax.com](https://speckit.tekimax.com)
- Repo: [TEKIMAX/speckit-security](https://github.com/TEKIMAX/speckit-security)

## What the extension is for

Based on the current upstream README, `speckit-security` is a Spec Kit extension that adds security gates for:

- data contracts
- threat modeling
- model governance
- AI guardrails
- implementation gate checks
- post-implementation audit
- red-team verification
- dependency CVE auditing

It is intended to complement, not replace, normal application security work.

## Recommended use on this project

Run the extension at two levels:

1. Per service while building:
   - Go ledger engine
   - Stripe webhook handler
   - forecast service
   - LLM service
   - Rust crypto utility
2. End-to-end once the full stack is connected.

## Installation steps

The current upstream README indicates this extension is installed into a Spec Kit project after Spec Kit is already available:

```bash
git clone https://github.com/TEKIMAX/speckit-security.git /tmp/speckit-security
cd /path/to/your-project
specify extension add --dev /tmp/speckit-security
specify extension list
```

## Suggested command flow

```bash
/speckit.specify
/speckit.tekimax-security.data-contract
/speckit.tekimax-security.guardrails
/speckit.plan
/speckit.tekimax-security.threat-model
/speckit.tekimax-security.model-governance
/speckit.tasks
/speckit.tekimax-security.gate-check
/speckit.implement
/speckit.tekimax-security.audit
/speckit.tekimax-security.red-team
/speckit.analyze
```

## Project-specific checklist before running the scan

- Confirm `.env` files are not committed
- Confirm internal service tokens are set through environment variables
- Confirm no inline prompts or secrets were added directly to code
- Confirm Python, Go, Node, and Rust dependencies are pinned and buildable
- Confirm the threat model and RAID docs are up to date

## Local verification to pair with speckit-security

Run these in addition to the extension:

```bash
docker compose build
docker compose up -d
./scripts/bootstrap_demo.sh
```

Then verify:

- `http://localhost:8080/healthz`
- `http://localhost:3001/health`
- `http://localhost:8001/health`
- `http://localhost:8002/health`

## Notes

- The extension depends on Spec Kit being installed first.
- If the team decides not to adopt Spec Kit for this class project workflow, keep this file as the documented integration path requested by management and continue with normal security review plus dependency and secret scanning.

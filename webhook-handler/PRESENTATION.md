# Presentation Script — Stripe Webhook Ingestion Pipeline

## Before You Start

1. Make sure the server is running: `npm run dev`
2. Open **http://localhost:3000** in your browser (the dashboard)
3. Have a second terminal ready to run `npx ts-node test-events.ts` for the live demo
4. Optionally have your IDE open to show code when walking through architecture

---

## Opening (1–2 minutes)

> "What I've built is a Stripe webhook ingestion module for a financial platform.
> The problem it solves is: when a customer pays, gets refunded, or an invoice is
> settled on Stripe, our platform needs to know about it — reliably, securely,
> and exactly once.
>
> Stripe sends us these notifications through webhooks — essentially POST
> requests to our server. But there are real engineering challenges here:
> How do you verify the event is actually from Stripe and not spoofed?
> What if Stripe sends the same event twice? How do you translate Stripe's
> data model into your own internal format? And how do you record the
> financial impact correctly?
>
> This module handles all of that. Let me walk you through it."

---

## Architecture Walkthrough (3–4 minutes)

> "Here's the pipeline at a high level."

**Show the dashboard or draw on a whiteboard:**

```
Stripe → Webhook Endpoint → Verify Signature → Store Raw Event
       → Check for Duplicates → Normalize → Validate → Post to Ledger
```

Walk through each step:

### 1. Signature Verification
> "When Stripe sends us a webhook, it signs the request body with a shared
> secret. We verify that signature before doing anything else. If it doesn't
> match, we reject the request with a 400. This prevents anyone from sending
> fake events to our endpoint.
>
> There's an important implementation detail here — we have to receive the
> raw bytes of the request body, not parsed JSON. If Express parses it first
> and we re-serialize it, the bytes might differ and the signature check fails.
> That's why this one route uses `express.raw()` instead of `express.json()`."

### 2. Idempotency
> "Stripe guarantees at-least-once delivery, which means they might send the
> same event more than once. If we process a charge twice, we'd double-count
> revenue. So we use Stripe's event ID as an idempotency key — before processing,
> we check if we've already seen this event. If we have, we return 200 and skip it.
>
> We also persist the raw event *before* processing, so if the server crashes
> between receiving and processing, we don't lose the event."

### 3. Normalization
> "Stripe has many different object types — charges, invoices, refunds — each
> with their own shape. We normalize all of them into one canonical
> `NormalizedTransaction` type with consistent fields: event ID, amount,
> currency, direction, customer. This decouples the rest of our platform from
> Stripe's API — if Stripe changes their schema, we only update one place."

### 4. Ledger Posting
> "Once normalized, we post the transaction to our ledger using double-entry
> bookkeeping. Every transaction creates two entries that balance to zero:
>
> - For an inflow like a charge: Debit Stripe Balance, Credit Revenue
> - For an outflow like a refund: Debit Contra-Revenue, Credit Stripe Balance
>
> Right now this is simulated, but the interface is ready for a real database."

---

## Live Demo (3–4 minutes)

> "Let me show you this working live."

**Point to the dashboard in the browser.**

> "This is a real-time dashboard connected to the server via Server-Sent Events.
> It shows every webhook event, its status, and the ledger journal entries."

**Point out the stats cards:**
> "At the top you can see summary stats — total events processed, how many
> were posted to the ledger, how many failed, how many were ignored because
> they're event types we don't handle, and how many duplicates we blocked."

**Point to the events table:**
> "Each row shows the Stripe event ID, the event type, a color-coded status
> badge, the customer, amount, and when it arrived."

**Point to the ledger journal:**
> "Down here you can see the actual double-entry journal entries with the
> debit and credit accounts and amounts."

**Now run the test events live. Switch to your terminal:**

```bash
npx ts-node test-events.ts
```

> "Watch the dashboard — these events are appearing in real time. The test
> script sends six events:"

| # | What happens | What to point out on screen |
|---|---|---|
| 1 | charge.succeeded ($49.99) | New row appears, status POSTED, ledger shows Debit Stripe Balance / Credit Revenue |
| 2 | charge.refunded (€12.00) | Direction is OUTFLOW, ledger flips: Debit Contra-Revenue / Credit Stripe Balance |
| 3 | invoice.paid ($299.00) | Different event type, same INFLOW treatment |
| 4 | Duplicate of event #1 | **No new row in events table** — duplicates counter increments. "This is idempotency in action." |
| 5 | customer.created (unsupported) | Status badge shows IGNORED in yellow. "We don't crash — we acknowledge it and move on." |
| 6 | Second charge.succeeded (£750.00 GBP) | New unique event, different currency — processes normally |

> "So in six requests you've seen the full lifecycle: successful processing,
> refund handling, duplicate detection, and graceful handling of unsupported
> event types."

---

## Code Quality Points (2 minutes)

If they want to see code, have these files ready:

| Point to make | File to show |
|---|---|
| Clean separation of concerns | `src/` folder structure — controller, services, repository, types |
| Repository pattern for swappable storage | `src/types/events.ts` — the `IEventRepository` interface + PostgreSQL schema in comments |
| Type safety | `src/types/transaction.ts` — the `NormalizedTransaction` interface |
| Dependency injection | `src/controllers/webhook.controller.ts` — `createWebhookHandler(repo)` takes the repo as a parameter |
| Production-ready error handling | Same file — try/catch marks events FAILED with the error message |

> "The in-memory storage is designed to be swapped out for PostgreSQL — every
> method returns a Promise, and the SQL schema is already documented in the
> code. You'd just write a new class implementing the same interface."

---

## Closing (1 minute)

> "To summarize: this module gives us a secure, idempotent pipeline for
> ingesting Stripe webhooks. It verifies every event, prevents duplicates,
> normalizes data into a clean internal format, and records double-entry
> ledger entries. The architecture is modular — you can swap the storage
> layer, add new event types, or replace the simulated ledger with a real
> one without touching the rest of the code.
>
> The live dashboard you saw is powered by Server-Sent Events, so in
> production you could monitor the pipeline in real time."

---

## Anticipated Questions & Answers

**Q: What happens if the server crashes mid-processing?**
> We persist the raw event before processing. On restart, any event stuck in
> RECEIVED status could be replayed. The idempotency check prevents
> double-processing.

**Q: Why not use a message queue like Kafka or RabbitMQ?**
> For this scope, direct processing is simpler and sufficient. But the
> architecture supports it — you'd swap `postToLedger` to publish to a
> queue instead.

**Q: How would you scale this?**
> Move to PostgreSQL for event storage (schema is already defined), add a
> queue between the webhook handler and the ledger service, and run multiple
> worker instances. The idempotency check at the database level would use
> a unique constraint on event_id.

**Q: Why in-memory storage instead of a real database?**
> This is a demo/architecture proof. The `IEventRepository` interface is the
> contract — swapping to Postgres requires only a new class, zero changes to
> the controller or services.

**Q: What's the `express.raw()` thing about?**
> Stripe signs the exact bytes they send. If Express parses the JSON first,
> re-serializing it may change whitespace or key ordering, breaking the
> signature. We need the raw bytes to verify.

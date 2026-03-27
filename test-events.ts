// ============================================================
// TEST SCRIPT — Sends fake Stripe webhook events to our server
// to demonstrate the pipeline working end-to-end.
//
// Since we're not using real Stripe credentials, we need to
// generate valid HMAC-SHA256 signatures ourselves. We use the
// same webhook secret ("whsec_placeholder") that's in our .env
// file. This is exactly how Stripe signs their webhooks — we're
// just doing it manually for testing.
//
// Run this with: npx ts-node test-events.ts
// (make sure the server is running first with: npm run dev)
// ============================================================

import crypto from "crypto";

// Must match the STRIPE_WEBHOOK_SECRET in your .env file.
const WEBHOOK_SECRET = "whsec_placeholder";
const BASE_URL = "http://localhost:3000/stripe/webhook";

// Creates a valid Stripe-style signature for a payload.
// This mimics exactly what Stripe does on their end:
// 1. Get the current timestamp
// 2. Concatenate: timestamp + "." + payload
// 3. HMAC-SHA256 hash it with the webhook secret
// 4. Format as: t=<timestamp>,v1=<hash>
function signPayload(payload: string): string {
  const timestamp = Math.floor(Date.now() / 1000);
  const signature = crypto
    .createHmac("sha256", WEBHOOK_SECRET)
    .update(`${timestamp}.${payload}`)
    .digest("hex");
  return `t=${timestamp},v1=${signature}`;
}

// Sends a single test event to our webhook endpoint.
// Signs it, POSTs it, and prints the response.
async function sendEvent(name: string, payload: object): Promise<void> {
  const body = JSON.stringify(payload);
  const sig = signPayload(body);

  const res = await fetch(BASE_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "stripe-signature": sig,           // Our computed signature
    },
    body,
  });

  const json = await res.json();
  console.log(`\n── ${name} ──`);
  console.log(`   Status: ${res.status}`);
  console.log(`   Response:`, json);
}

async function main() {
  // ── EVENT 1: Successful charge ──────────────────────────────
  // A customer paid $49.99. This should be processed as an
  // INFLOW and create a "Debit Stripe Balance / Credit Revenue"
  // ledger entry.
  await sendEvent("charge.succeeded", {
    id: "evt_charge_success_001",
    object: "event",
    type: "charge.succeeded",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "ch_1ABC",
        object: "charge",
        amount: 4999,                    // $49.99 in cents
        currency: "usd",
        customer: "cus_TestCustomer1",
        status: "succeeded",
      },
    },
  });

  // ── EVENT 2: Refund ─────────────────────────────────────────
  // A charge of €12.00 was refunded. This should be processed
  // as an OUTFLOW and create a "Debit Contra-Revenue / Credit
  // Stripe Balance" ledger entry.
  await sendEvent("charge.refunded", {
    id: "evt_charge_refund_002",
    object: "event",
    type: "charge.refunded",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "ch_2DEF",
        object: "charge",
        amount: 1200,                    // €12.00 in cents
        currency: "eur",
        customer: "cus_TestCustomer2",
        status: "succeeded",
      },
    },
  });

  // ── EVENT 3: Invoice paid ───────────────────────────────────
  // A subscription invoice of $299.00 was paid. Processed as
  // an INFLOW, similar to a charge.
  await sendEvent("invoice.paid", {
    id: "evt_invoice_paid_003",
    object: "event",
    type: "invoice.paid",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "in_3GHI",
        object: "invoice",
        amount_paid: 29900,              // $299.00 in cents
        currency: "usd",
        customer: "cus_TestCustomer3",
        status: "paid",
      },
    },
  });

  // ── EVENT 4: Duplicate (same ID as Event 1) ────────────────
  // This has the SAME event ID as Event 1 above. Our
  // idempotency check should catch this and return
  // { duplicate: true } without processing it again.
  // This proves we won't double-count revenue.
  await sendEvent("charge.succeeded (DUPLICATE)", {
    id: "evt_charge_success_001",        // Same ID as Event 1!
    object: "event",
    type: "charge.succeeded",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "ch_1ABC",
        object: "charge",
        amount: 4999,
        currency: "usd",
        customer: "cus_TestCustomer1",
        status: "succeeded",
      },
    },
  });

  // ── EVENT 5: Unsupported event type ─────────────────────────
  // "customer.created" is not in our supported events list.
  // Our server should acknowledge it (200 OK) but mark it
  // as IGNORED without trying to process it. This proves
  // we handle unknown event types gracefully.
  await sendEvent("customer.created (UNSUPPORTED)", {
    id: "evt_unsupported_004",
    object: "event",
    type: "customer.created",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "cus_NewCustomer",
        object: "customer",
        email: "test@example.com",
      },
    },
  });

  // ── EVENT 6: Another successful charge (unique) ─────────────
  // A different charge for £750.00 GBP with no customer attached.
  // Different event ID than Event 1, so this is NOT a duplicate.
  // Shows the system handles multiple currencies and null customers.
  await sendEvent("charge.succeeded (second unique)", {
    id: "evt_charge_success_005",        // Different ID = new event
    object: "event",
    type: "charge.succeeded",
    created: Math.floor(Date.now() / 1000),
    data: {
      object: {
        id: "ch_4JKL",
        object: "charge",
        amount: 75000,                   // £750.00 in pence
        currency: "gbp",
        customer: null,                  // No customer attached
        status: "succeeded",
      },
    },
  });

  console.log("\n✔ All test events sent.\n");
}

main().catch(console.error);

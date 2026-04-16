// ============================================================
// WEBHOOK CONTROLLER — The heart of the pipeline.
//
// This is the code that runs every time Stripe sends us a
// webhook event. It handles the full lifecycle:
//
//   1. Verify the signature (is this really from Stripe?)
//   2. Check for duplicates (have we seen this event before?)
//   3. Save the raw event (persist before processing)
//   4. Route by type (do we support this event type?)
//   5. Normalize the data (convert to our internal format)
//   6. Validate the data (sanity checks)
//   7. Post to the ledger (record the financial transaction)
//   8. Update the event status (mark as posted, failed, etc.)
//
// ============================================================
//
// PIPELINE FLOW (Mermaid diagram):
//
//   Stripe
//     |
//     v
//   POST /stripe/webhook
//     |
//     v
//   Verify Signature ── fail ──> 400 Bad Request
//     |
//     v (pass)
//   Check Duplicate ── yes ──> 200 "duplicate: true"
//     |
//     v (new event)
//   Save Raw Event
//     |
//     v
//   Supported Type? ── no ──> Mark IGNORED, 200
//     |
//     v (yes)
//   Normalize to NormalizedTransaction
//     |
//     v
//   Validate (amount > 0, etc.)
//     |
//     v
//   Post to Ledger
//     |
//     v
//   Mark POSTED, 200
//
// If anything fails during steps 5-7, we catch the error,
// mark the event as FAILED, and return 500.
// ============================================================

import { Request, Response } from "express";
import { verifyWebhookSignature } from "../services/stripe.service";
import {
  isSupportedEvent,
  normalizeStripeEvent,
} from "../services/normalization.service";
import { postToLedger } from "../services/ledger.service";
import { IEventRepository } from "../types";
import { eventBus } from "../services/event-bus";

// This function creates the webhook handler. We use a "factory"
// pattern so we can pass in the event repository — this is called
// "dependency injection" and it makes the code easy to test and
// easy to swap storage backends.
export function createWebhookHandler(repo: IEventRepository) {
  return async function handleWebhook(
    req: Request,
    res: Response
  ): Promise<void> {

    // ── STEP 1: Verify the signature ──────────────────────────
    // Every Stripe webhook includes a "stripe-signature" header.
    // We use it to verify the request really came from Stripe
    // and wasn't tampered with in transit.

    const signature = req.headers["stripe-signature"];

    // If there's no signature header at all, reject immediately.
    if (!signature || typeof signature !== "string") {
      res.status(400).json({ error: "Missing stripe-signature header" });
      return;
    }

    // Try to verify the signature and parse the event.
    // If the signature is invalid, Stripe's SDK throws an error.
    let event;
    try {
      event = verifyWebhookSignature(req.body as Buffer, signature);
    } catch (err) {
      console.warn("[Webhook] Signature verification failed:", err);
      res.status(400).json({ error: "Invalid signature" });
      return;
    }

    console.log(`[Webhook] Received event ${event.id} (${event.type})`);

    // ── STEP 2: Idempotency check ────────────────────────────
    // Stripe may deliver the same event more than once due to
    // network retries or their at-least-once delivery guarantee.
    //
    // If we processed the same charge.succeeded twice, we'd
    // record the revenue twice — that would be a serious bug.
    //
    // So we use the event.id as a unique key. If we've already
    // seen this ID, we skip processing entirely and tell Stripe
    // "yes, we got it" (200 OK) so it stops retrying.
    const alreadySeen = await repo.hasSeenEvent(event.id);
    if (alreadySeen) {
      console.log(`[Webhook] Duplicate event ${event.id} — skipping`);
      // Notify the dashboard that a duplicate was blocked.
      eventBus.emit("duplicate", event.id);
      res.status(200).json({ received: true, duplicate: true });
      return;
    }

    // ── STEP 3: Persist the raw event ────────────────────────
    // We save the event BEFORE processing it. This is important:
    // if the server crashes during processing, we still have the
    // raw event saved and could retry later.
    await repo.saveRawEvent(event);

    // ── STEP 4: Check if we support this event type ──────────
    // Stripe sends many event types (charge.succeeded, customer.created,
    // payment_intent.created, etc.). We only process a few of them.
    // Unsupported types get acknowledged but not processed.
    if (!isSupportedEvent(event.type)) {
      console.log(`[Webhook] Ignoring unsupported event type: ${event.type}`);
      await repo.markEventIgnored(event.id);
      // Notify the dashboard about this ignored event.
      const stored = await repo.getEvent(event.id);
      if (stored) eventBus.emit("webhookEvent", stored);
      res.status(200).json({ received: true, ignored: true });
      return;
    }

    // ── STEP 5: Normalize, Validate, and Post to Ledger ──────
    // This is where the actual business logic happens.
    try {
      // Convert the raw Stripe event into our internal format.
      const transaction = normalizeStripeEvent(event);

      // Basic validation: the amount should be positive.
      if (transaction.amount <= 0) {
        throw new Error(
          `Invalid amount ${transaction.amount} for event ${event.id}`
        );
      }

      // Record the financial transaction in the ledger
      // (double-entry bookkeeping).
      await postToLedger(transaction);

      // Mark the event as successfully processed.
      await repo.markEventPosted(event.id);

      // Notify the dashboard about this newly processed event.
      const stored = await repo.getEvent(event.id);
      if (stored) eventBus.emit("webhookEvent", stored);

      console.log(`[Webhook] Successfully processed event ${event.id}`);
      res.status(200).json({ received: true, processed: true });

    } catch (err) {
      // Something went wrong during processing. We don't crash
      // the server — instead we mark the event as FAILED and
      // record what went wrong so we can investigate later.
      const message =
        err instanceof Error ? err.message : "Unknown processing error";
      console.error(`[Webhook] Processing failed for ${event.id}:`, message);
      await repo.markEventFailed(event.id, message);

      // Still notify the dashboard so it shows the failed event.
      const stored = await repo.getEvent(event.id);
      if (stored) eventBus.emit("webhookEvent", stored);

      res.status(500).json({ error: "Event processing failed" });
    }
  };
}

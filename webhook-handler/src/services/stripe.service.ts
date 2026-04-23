// ============================================================
// STRIPE SERVICE — Handles communication with the Stripe API.
//
// Right now its main job is verifying webhook signatures.
// In a larger app, this is also where you'd put functions for
// creating charges, issuing refunds, etc.
// ============================================================

import Stripe from "stripe";
import { config } from "../config";

let stripe: Stripe | null = null;

if (config.stripe.secretKey) {
  // Create a single Stripe client instance that's shared across
  // the whole app when Stripe is configured for the demo.
  stripe = new Stripe(config.stripe.secretKey, {
    apiVersion: "2026-02-25.clover",
  });
}

export function isStripeConfigured(): boolean {
  return Boolean(stripe && config.stripe.webhookSecret);
}

// ============================================================
// SIGNATURE VERIFICATION
//
// WHY THIS MATTERS:
// Anyone on the internet could send a POST request to our
// /stripe/webhook endpoint and pretend to be Stripe. Without
// signature verification, we'd blindly trust fake data.
//
// HOW IT WORKS:
// 1. Stripe computes an HMAC-SHA256 hash of the request body
//    using our shared webhook secret.
// 2. Stripe puts that hash in the "stripe-signature" header.
// 3. We recompute the hash ourselves and compare.
// 4. If they match, the request genuinely came from Stripe.
//
// WHY RAW BODY?
// This only works if we hash the EXACT same bytes Stripe did.
// If Express parses the JSON and we re-serialize it, the bytes
// might differ (different spacing, different key ordering).
// That's why we use express.raw() to keep the original bytes.
// ============================================================
export function verifyWebhookSignature(
  rawBody: Buffer,
  signature: string
): Stripe.Event {
  if (!stripe || !config.stripe.webhookSecret) {
    throw new Error("Stripe is not configured");
  }

  // This Stripe SDK function does all the heavy lifting:
  // it checks the signature, verifies the timestamp isn't too old
  // (to prevent replay attacks), and returns the parsed event
  // if everything checks out. If not, it throws an error.
  return stripe.webhooks.constructEvent(
    rawBody,
    signature,
    config.stripe.webhookSecret
  );
}

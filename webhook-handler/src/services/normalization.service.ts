// ============================================================
// NORMALIZATION SERVICE — Converts raw Stripe events into our
// internal NormalizedTransaction format.
//
// Stripe sends dozens of different event types, each with their
// own data structure. This service is the "translator" that
// takes the Stripe-specific data and maps it into a consistent
// shape our platform understands.
//
// Supported event types and what they map to:
//   charge.succeeded  -> type "charge",          direction "INFLOW"
//   charge.refunded   -> type "refund",           direction "OUTFLOW"
//   invoice.paid      -> type "invoice_payment",  direction "INFLOW"
// ============================================================

import Stripe from "stripe";
import { NormalizedTransaction } from "../types";

// The set of event types we know how to process.
// Anything not in this set gets safely ignored.
const SUPPORTED_EVENTS = new Set([
  "charge.succeeded",
  "charge.refunded",
  "invoice.paid",
]);

// Quick check: is this an event type we can handle?
export function isSupportedEvent(eventType: string): boolean {
  return SUPPORTED_EVENTS.has(eventType);
}

// Main entry point: takes a raw Stripe event, figures out what
// type it is, and calls the appropriate converter function.
export function normalizeStripeEvent(
  event: Stripe.Event
): NormalizedTransaction {
  switch (event.type) {
    // A customer's payment went through successfully.
    // Money is coming IN to our platform.
    case "charge.succeeded":
      return normalizeCharge(event, "charge", "INFLOW");

    // A charge was refunded — we're giving money back.
    // Money is going OUT of our platform.
    case "charge.refunded":
      return normalizeCharge(event, "refund", "OUTFLOW");

    // A subscription invoice was paid.
    // Money is coming IN to our platform.
    case "invoice.paid":
      return normalizeInvoice(event);

    // This should never happen because we check isSupportedEvent()
    // first, but just in case, throw a clear error.
    default:
      throw new Error(`Unsupported event type: ${event.type}`);
  }
}

// Converts a charge event (either succeeded or refunded) into
// our normalized format. Both use the same Stripe Charge object,
// just with different type and direction labels.
function normalizeCharge(
  event: Stripe.Event,
  type: "charge" | "refund",
  direction: "INFLOW" | "OUTFLOW"
): NormalizedTransaction {
  // Tell TypeScript the data object is specifically a Charge.
  const charge = event.data.object as Stripe.Charge;

  return {
    stripeEventId: event.id,           // e.g. "evt_abc123"
    stripeObjectId: charge.id,         // e.g. "ch_xyz789"
    type,                              // "charge" or "refund"
    direction,                         // "INFLOW" or "OUTFLOW"
    amount: charge.amount,             // Amount in cents (e.g. 4999 = $49.99)
    currency: charge.currency,         // e.g. "usd"
    occurredAt: new Date(event.created * 1000), // Stripe uses Unix seconds, JS uses milliseconds

    // The customer field can be a string ID or a full Customer object.
    // We always want just the string ID.
    customerId: (typeof charge.customer === "string"
      ? charge.customer
      : charge.customer?.id) ?? null,
  };
}

// Converts an invoice.paid event into our normalized format.
// Invoices have slightly different fields than charges
// (e.g. amount_paid instead of amount).
function normalizeInvoice(event: Stripe.Event): NormalizedTransaction {
  const invoice = event.data.object as Stripe.Invoice;

  return {
    stripeEventId: event.id,
    stripeObjectId: invoice.id,        // e.g. "in_abc456"
    type: "invoice_payment",
    direction: "INFLOW",               // Paid invoices are always money coming in
    amount: invoice.amount_paid,       // How much was actually paid (in cents)
    currency: invoice.currency,
    occurredAt: new Date(event.created * 1000),
    customerId: (typeof invoice.customer === "string"
      ? invoice.customer
      : invoice.customer?.id) ?? null,
  };
}

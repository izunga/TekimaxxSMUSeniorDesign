// ============================================================
// NORMALIZED TRANSACTION — The single, consistent data shape
// that every Stripe event gets converted into.
//
// Think of this as our "universal translator." Stripe has many
// different object types (charges, invoices, refunds) each with
// their own structure. We convert all of them into this ONE
// shape so the rest of our platform doesn't need to know
// anything about Stripe's specific data format.
//
// If Stripe changes their API tomorrow, we only need to update
// the normalization logic — everything downstream stays the same.
// ============================================================

export interface NormalizedTransaction {
  // The unique ID Stripe assigned to this event (e.g. "evt_abc123").
  // We also use this as our idempotency key to prevent duplicates.
  stripeEventId: string;

  // The ID of the actual object inside the event — the charge ID,
  // invoice ID, etc. (e.g. "ch_xyz789" or "in_abc456").
  stripeObjectId: string;

  // What kind of transaction this is in our system.
  // "charge" = someone paid us, "refund" = we gave money back,
  // "invoice_payment" = a subscription invoice was paid.
  type: TransactionType;

  // Which way the money is flowing relative to our platform.
  // "INFLOW" = money coming in (charges, invoice payments).
  // "OUTFLOW" = money going out (refunds).
  direction: TransactionDirection;

  // The dollar amount in the smallest currency unit.
  // For USD, this is cents. So $49.99 = 4999.
  // We store it this way to avoid floating point math issues.
  amount: number;

  // The three-letter currency code, lowercased (e.g. "usd", "eur").
  // Follows the ISO 4217 standard.
  currency: string;

  // When this event actually happened on Stripe's side.
  // This might be slightly different from when we received it.
  occurredAt: Date;

  // The Stripe customer ID if one was attached to the transaction.
  // null if there's no customer (e.g. a one-off charge with no account).
  customerId: string | null;
}

// The three types of transactions our system understands.
export type TransactionType = "charge" | "refund" | "invoice_payment";

// Money either flows IN to our platform or OUT of it.
export type TransactionDirection = "INFLOW" | "OUTFLOW";

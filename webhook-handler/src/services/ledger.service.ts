// ============================================================
// LEDGER SERVICE — Records financial transactions using
// double-entry bookkeeping.
//
// WHAT IS DOUBLE-ENTRY BOOKKEEPING?
// It's the standard accounting method used by every real
// business. Every financial transaction creates TWO entries
// that balance each other out:
//
//   - A DEBIT entry (money coming into an account)
//   - A CREDIT entry (money leaving an account)
//
// The two entries always equal the same amount, so the books
// always balance. This is how accountants catch errors.
//
// HOW WE USE IT:
//
//   When money comes IN (charge, invoice payment):
//     Debit  "Stripe Balance"  (our asset account goes UP)
//     Credit "Revenue"         (our income goes UP)
//
//   When money goes OUT (refund):
//     Debit  "Contra-Revenue"  (our expenses go UP)
//     Credit "Stripe Balance"  (our asset account goes DOWN)
//
// Right now this is simulated (we just log and store in memory).
// In production, these entries would be written to a database.
// ============================================================

import { NormalizedTransaction } from "../types";
import { eventBus, LedgerEntry } from "./event-bus";
import { config } from "../config";

// In-memory list of all ledger journal entries.
// The dashboard reads from this to display the journal.
const ledgerEntries: LedgerEntry[] = [];

// Takes a normalized transaction and creates the double-entry
// journal entries for it.
export async function postToLedger(tx: NormalizedTransaction): Promise<void> {
  // Format the amount for display (e.g. 4999 cents -> "49.99 USD")
  const amountFormatted = formatAmount(tx.amount, tx.currency);

  // Decide which accounts to debit and credit based on
  // whether money is coming in or going out.
  let debitAccount: string;
  let creditAccount: string;

  if (tx.direction === "INFLOW") {
    // Money coming in: our Stripe balance increases,
    // and we record revenue.
    debitAccount = "Stripe Balance";
    creditAccount = "Revenue";
  } else {
    // Money going out (refund): we record an expense
    // (contra-revenue) and our Stripe balance decreases.
    debitAccount = "Contra-Revenue";
    creditAccount = "Stripe Balance";
  }

  // Log the journal entry to the terminal for visibility.
  console.log(
    `[Ledger] JOURNAL for ${tx.type} (${tx.stripeEventId}):\n` +
      `         Debit  ${debitAccount.padEnd(18)} ${amountFormatted}\n` +
      `         Credit ${creditAccount.padEnd(18)} ${amountFormatted}`
  );

  // Build the ledger entry object.
  const entry: LedgerEntry = {
    eventId: tx.stripeEventId,
    type: tx.type,
    direction: tx.direction,
    debitAccount,
    creditAccount,
    amount: tx.amount,
    currency: tx.currency,
    timestamp: new Date(),
  };

  // Store it in memory so the dashboard can display it.
  ledgerEntries.push(entry);

  // Notify the event bus so any connected dashboard gets
  // a real-time update via Server-Sent Events.
  eventBus.emit("ledgerEntry", entry);

  // Persist to the Go ledger engine if account IDs are configured.
  // Without account IDs the in-memory store above is the only record.
  await persistToLedgerEngine(tx);
}

// Attempts an HTTP POST to the Go ledger engine's POST /transactions endpoint.
// Requires LEDGER_SERVICE_TOKEN and the three account ID env vars to be set.
// Fails gracefully — a failed call is logged but never throws.
async function persistToLedgerEngine(tx: NormalizedTransaction): Promise<void> {
  const { engineUrl, serviceToken, stripeBalanceAccountId, revenueAccountId, contraRevenueAccountId } = config.ledger;

  if (!serviceToken || !stripeBalanceAccountId || !revenueAccountId || !contraRevenueAccountId) {
    // Not fully configured — skip silently. Set the four env vars to enable persistence.
    return;
  }

  const debitAccountId  = tx.direction === "INFLOW" ? stripeBalanceAccountId : contraRevenueAccountId;
  const creditAccountId = tx.direction === "INFLOW" ? revenueAccountId       : stripeBalanceAccountId;

  try {
    const res = await fetch(`${engineUrl}/transactions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${serviceToken}`,
      },
      body: JSON.stringify({
        source: "stripe",
        external_reference: tx.stripeEventId,
        description: `${tx.type} ${tx.stripeObjectId}`,
        lines: [
          { account_id: debitAccountId,  debit: tx.amount, credit: 0 },
          { account_id: creditAccountId, debit: 0,         credit: tx.amount },
        ],
      }),
    });
    if (!res.ok) {
      console.error(`[Ledger] Ledger engine rejected transaction ${tx.stripeEventId}: HTTP ${res.status}`);
    }
  } catch (err) {
    console.error(`[Ledger] Failed to reach ledger engine for ${tx.stripeEventId}:`, err);
  }
}

// Returns a copy of all ledger entries (used by the dashboard API).
export function getLedgerEntries(): LedgerEntry[] {
  return [...ledgerEntries];
}

// Helper: converts an amount in minor units (cents) to a
// human-readable string like "49.99 USD".
function formatAmount(amountMinor: number, currency: string): string {
  const major = (amountMinor / 100).toFixed(2);
  return `${major} ${currency.toUpperCase()}`;
}

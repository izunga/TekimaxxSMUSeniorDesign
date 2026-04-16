// ============================================================
// EVENT BUS — A simple publish/subscribe messaging system that
// lets different parts of the app communicate without being
// directly connected to each other.
//
// HOW IT'S USED:
// 1. The webhook controller EMITS events when webhooks arrive
//    (e.g. "a new event was processed" or "a duplicate was blocked").
// 2. The dashboard SSE endpoint LISTENS for those events and
//    pushes them to connected browsers in real time.
//
// This decoupling means the webhook controller doesn't need to
// know anything about the dashboard, and vice versa. They just
// communicate through the bus.
//
// Think of it like a radio station: the webhook controller
// broadcasts, and any number of dashboard connections can tune in.
// ============================================================

import { EventEmitter } from "events";
import { StoredEvent } from "../types";

// Shape of a ledger journal entry (used for real-time updates).
export interface LedgerEntry {
  eventId: string;              // Which Stripe event this belongs to
  type: string;                 // "charge", "refund", or "invoice_payment"
  direction: "INFLOW" | "OUTFLOW";
  debitAccount: string;         // e.g. "Stripe Balance"
  creditAccount: string;        // e.g. "Revenue"
  amount: number;               // Amount in cents
  currency: string;             // e.g. "usd"
  timestamp: Date;              // When this entry was created
}

// Defines the three types of messages the bus can carry,
// and what data each one includes.
interface BusEvents {
  // Fired when a webhook event is processed, ignored, or failed.
  webhookEvent: (event: StoredEvent) => void;

  // Fired when a new ledger journal entry is created.
  ledgerEntry: (entry: LedgerEntry) => void;

  // Fired when a duplicate event is detected and skipped.
  duplicate: (eventId: string) => void;
}

// A type-safe wrapper around Node.js EventEmitter.
// The generic types ensure you can only emit/listen to
// the events defined in BusEvents above.
class WebhookEventBus {
  private emitter = new EventEmitter();

  // Subscribe to an event (start listening).
  on<K extends keyof BusEvents>(event: K, listener: BusEvents[K]): void {
    this.emitter.on(event, listener as (...args: unknown[]) => void);
  }

  // Unsubscribe from an event (stop listening).
  off<K extends keyof BusEvents>(event: K, listener: BusEvents[K]): void {
    this.emitter.off(event, listener as (...args: unknown[]) => void);
  }

  // Broadcast an event to all listeners.
  emit<K extends keyof BusEvents>(
    event: K,
    ...args: Parameters<BusEvents[K]>
  ): void {
    this.emitter.emit(event, ...args);
  }
}

// Single shared instance used by the whole app.
export const eventBus = new WebhookEventBus();

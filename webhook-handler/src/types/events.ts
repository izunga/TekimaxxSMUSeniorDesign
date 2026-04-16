// ============================================================
// EVENT TYPES — Defines how we track the lifecycle of every
// Stripe webhook event that arrives at our server.
//
// Every event goes through a lifecycle:
//   1. RECEIVED  — We saved the raw event, haven't processed it yet
//   2. POSTED    — Successfully converted and sent to the ledger
//   3. FAILED    — We tried to process it but something went wrong
//   4. IGNORED   — It's an event type we don't handle (e.g. customer.created)
// ============================================================

import Stripe from "stripe";

// The four possible states an event can be in.
export type EventStatus = "RECEIVED" | "POSTED" | "FAILED" | "IGNORED";

// What we store for each event — the raw Stripe data plus our
// own metadata about when we received it and what happened.
export interface StoredEvent {
  eventId: string;            // Stripe's unique event ID
  status: EventStatus;        // Current lifecycle state
  rawEvent: Stripe.Event;     // The complete original event from Stripe
  receivedAt: Date;           // When our server first received this event
  processedAt: Date | null;   // When we finished processing (null if still pending)
  failureReason: string | null; // If it failed, why? (null if it didn't fail)
}

// ============================================================
// REPOSITORY INTERFACE — The "contract" for event storage.
//
// This is the key to making storage swappable. Any class that
// implements these methods can be plugged in — whether it uses
// an in-memory Map (like we do now), PostgreSQL, Redis, etc.
//
// The rest of the app only talks to this interface, never to
// the specific storage implementation directly.
// ============================================================
export interface IEventRepository {
  // Save the raw Stripe event as soon as it arrives.
  // We do this BEFORE processing so if the server crashes,
  // we still have the event and can retry later.
  saveRawEvent(event: Stripe.Event): Promise<void>;

  // Check if we've already received an event with this ID.
  // This is how we prevent duplicate processing.
  hasSeenEvent(eventId: string): Promise<boolean>;

  // Mark an event as successfully posted to the ledger.
  markEventPosted(eventId: string): Promise<void>;

  // Mark an event as failed, with a reason explaining what went wrong.
  markEventFailed(eventId: string, reason: string): Promise<void>;

  // Mark an event as ignored (unsupported event type).
  markEventIgnored(eventId: string): Promise<void>;

  // Retrieve a single event by its ID.
  getEvent(eventId: string): Promise<StoredEvent | undefined>;

  // Get all stored events (used by the dashboard to show the full list).
  getAllEvents(): Promise<StoredEvent[]>;
}

/*
 * ──────────────────────────────────────────────────────────────
 * FUTURE: PostgreSQL Table Schema
 *
 * When we're ready to move from in-memory to a real database,
 * this is the SQL table we'd create. It mirrors the StoredEvent
 * interface above exactly.
 *
 *   CREATE TABLE stripe_events (
 *     event_id       TEXT PRIMARY KEY,           -- Stripe event.id
 *     status         TEXT NOT NULL DEFAULT 'RECEIVED',
 *     raw_event      JSONB NOT NULL,             -- Full event as JSON
 *     received_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
 *     processed_at   TIMESTAMPTZ,
 *     failure_reason TEXT
 *   );
 *
 *   -- Index for quickly finding events by status
 *   -- (e.g. "show me all FAILED events")
 *   CREATE INDEX idx_stripe_events_status ON stripe_events (status);
 * ──────────────────────────────────────────────────────────────
 */

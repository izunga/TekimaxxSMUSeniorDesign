// ============================================================
// IN-MEMORY EVENT REPOSITORY — Stores Stripe events in a
// JavaScript Map (basically a dictionary/hash table in RAM).
//
// This is a simple implementation meant for development and
// demos. In production, you'd swap this for a PostgreSQL or
// Redis-backed version. The key point is: this class implements
// the IEventRepository interface, so any replacement just needs
// to implement the same methods and the rest of the app doesn't
// need to change at all.
//
// All methods are async (return Promises) even though Map
// operations are synchronous — this way the callers already
// use async/await, making the swap to a real database seamless.
// ============================================================

import Stripe from "stripe";
import { IEventRepository, StoredEvent } from "../types";

export class InMemoryEventRepository implements IEventRepository {
  // A Map is like a dictionary: event ID -> stored event data.
  // This is where all events live while the server is running.
  // Note: everything is lost when the server restarts (that's
  // the trade-off of in-memory storage).
  private readonly store = new Map<string, StoredEvent>();

  // Save the raw Stripe event with a "RECEIVED" status.
  // This is the FIRST thing we do when a webhook arrives,
  // even before processing. That way if something crashes
  // during processing, we still have the event saved.
  async saveRawEvent(event: Stripe.Event): Promise<void> {
    this.store.set(event.id, {
      eventId: event.id,
      status: "RECEIVED",
      rawEvent: event,
      receivedAt: new Date(),
      processedAt: null,
      failureReason: null,
    });
  }

  // Check if we already have an event with this ID in our store.
  // This is the idempotency check — if it returns true,
  // we skip processing to avoid double-counting.
  async hasSeenEvent(eventId: string): Promise<boolean> {
    return this.store.has(eventId);
  }

  // Update the event's status to "POSTED" — meaning we
  // successfully normalized it and sent it to the ledger.
  async markEventPosted(eventId: string): Promise<void> {
    const entry = this.store.get(eventId);
    if (entry) {
      entry.status = "POSTED";
      entry.processedAt = new Date();
    }
  }

  // Update the event's status to "FAILED" and record why.
  // This helps with debugging — you can see exactly which
  // events failed and what the error message was.
  async markEventFailed(eventId: string, reason: string): Promise<void> {
    const entry = this.store.get(eventId);
    if (entry) {
      entry.status = "FAILED";
      entry.processedAt = new Date();
      entry.failureReason = reason;
    }
  }

  // Update the event's status to "IGNORED" — this happens
  // when Stripe sends us an event type we don't handle
  // (like customer.created). We acknowledge it but don't process it.
  async markEventIgnored(eventId: string): Promise<void> {
    const entry = this.store.get(eventId);
    if (entry) {
      entry.status = "IGNORED";
      entry.processedAt = new Date();
    }
  }

  // Look up a single event by its ID.
  async getEvent(eventId: string): Promise<StoredEvent | undefined> {
    return this.store.get(eventId);
  }

  // Return ALL events, sorted so the newest ones come first.
  // Used by the dashboard to display the full event list.
  async getAllEvents(): Promise<StoredEvent[]> {
    return Array.from(this.store.values()).sort(
      (a, b) => b.receivedAt.getTime() - a.receivedAt.getTime()
    );
  }
}

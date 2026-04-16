// ============================================================
// DASHBOARD CONTROLLER — API endpoints that power the web UI.
//
// Three routes:
//   GET /api/events  — Returns all stored webhook events as JSON
//   GET /api/ledger  — Returns all ledger journal entries as JSON
//   GET /api/stream  — Server-Sent Events (SSE) for real-time updates
//
// WHAT IS SSE (Server-Sent Events)?
// It's a way for the server to push updates to the browser
// without the browser having to keep asking "anything new?"
// The browser opens a long-lived connection, and the server
// sends messages down it whenever something happens. It's
// simpler than WebSockets and perfect for one-way updates
// like our dashboard.
// ============================================================

import { Request, Response, Router } from "express";
import { IEventRepository } from "../types";
import { getLedgerEntries } from "../services/ledger.service";
import { eventBus, LedgerEntry } from "../services/event-bus";
import { StoredEvent } from "../types";

// Converts an internal StoredEvent into a clean JSON object
// for the API response. We pick out just the fields the
// dashboard needs, and format dates as ISO strings.
function serializeEvent(e: StoredEvent) {
  // The raw Stripe event's data object could be any type
  // (charge, invoice, etc.), so we cast it to a generic
  // record to safely pull out common fields.
  const obj = e.rawEvent.data.object as unknown as Record<string, unknown>;
  return {
    eventId: e.eventId,
    status: e.status,
    type: e.rawEvent.type,
    receivedAt: e.receivedAt.toISOString(),
    processedAt: e.processedAt?.toISOString() ?? null,
    failureReason: e.failureReason,
    // Different Stripe objects use different field names for the amount.
    // Charges use "amount", invoices use "amount_paid".
    amount: obj.amount ?? obj.amount_paid ?? null,
    currency: obj.currency ?? null,
    customer: obj.customer ?? null,
  };
}

export function createDashboardRouter(repo: IEventRepository): Router {
  const router = Router();

  // ── GET /api/events ───────────────────────────────────────
  // Returns the full list of webhook events with their statuses.
  // The dashboard calls this on initial page load.
  router.get("/api/events", async (_req: Request, res: Response) => {
    const events = await repo.getAllEvents();
    res.json(events.map(serializeEvent));
  });

  // ── GET /api/ledger ───────────────────────────────────────
  // Returns all ledger journal entries.
  // The dashboard calls this on initial page load.
  router.get("/api/ledger", (_req: Request, res: Response) => {
    res.json(getLedgerEntries());
  });

  // ── GET /api/stream ───────────────────────────────────────
  // Server-Sent Events endpoint. The browser opens this
  // connection and keeps it open. Every time a webhook is
  // processed, we push the data down this connection so the
  // dashboard updates in real time without refreshing.
  router.get("/api/stream", (req: Request, res: Response) => {
    // Tell the browser this is an SSE stream, not a normal response.
    res.setHeader("Content-Type", "text/event-stream");
    // Don't cache this — we want live data.
    res.setHeader("Cache-Control", "no-cache");
    // Keep the connection open.
    res.setHeader("Connection", "keep-alive");
    // Send the headers immediately so the browser knows
    // the connection is established.
    res.flushHeaders();

    // When a webhook event is processed, send it to this
    // browser along with updated stats.
    const onEvent = async (stored: StoredEvent) => {
      const allEvents = await repo.getAllEvents();
      const stats = computeStats(allEvents);
      // SSE format: "event: <type>\ndata: <json>\n\n"
      res.write(
        `event: webhookEvent\ndata: ${JSON.stringify(serializeEvent(stored))}\n\n`
      );
      res.write(`event: stats\ndata: ${JSON.stringify(stats)}\n\n`);
    };

    // When a new ledger entry is created, send it to this browser.
    const onLedger = (entry: LedgerEntry) => {
      res.write(
        `event: ledgerEntry\ndata: ${JSON.stringify(entry)}\n\n`
      );
    };

    // When a duplicate event is blocked, notify this browser
    // so the "Duplicates Blocked" counter can update.
    const onDuplicate = async (eventId: string) => {
      const allEvents = await repo.getAllEvents();
      const stats = computeStats(allEvents);
      stats.duplicatesBlocked++;
      res.write(
        `event: duplicate\ndata: ${JSON.stringify({ eventId })}\n\n`
      );
      res.write(`event: stats\ndata: ${JSON.stringify(stats)}\n\n`);
    };

    // Subscribe to events from the event bus.
    eventBus.on("webhookEvent", onEvent);
    eventBus.on("ledgerEntry", onLedger);
    eventBus.on("duplicate", onDuplicate);

    // When the browser disconnects (closes tab, navigates away),
    // clean up our listeners so we don't leak memory.
    req.on("close", () => {
      eventBus.off("webhookEvent", onEvent);
      eventBus.off("ledgerEntry", onLedger);
      eventBus.off("duplicate", onDuplicate);
    });
  });

  return router;
}

// Counts up event statuses for the dashboard summary cards.
function computeStats(events: StoredEvent[]) {
  return {
    total: events.length,
    posted: events.filter((e) => e.status === "POSTED").length,
    failed: events.filter((e) => e.status === "FAILED").length,
    ignored: events.filter((e) => e.status === "IGNORED").length,
    duplicatesBlocked: 0, // This gets incremented by the onDuplicate handler
  };
}

// ============================================================
// APP FACTORY — Builds and configures the Express application.
//
// This is separated from index.ts so we can import and test
// the app without actually starting the HTTP server.
// ============================================================

import path from "path";
import express from "express";
import { createWebhookHandler } from "./controllers/webhook.controller";
import { createDashboardRouter } from "./controllers/dashboard.controller";
import { config, isStripeExpected } from "./config";
import { FileEventRepository } from "./repositories/file-event.repository";
import { InMemoryEventRepository } from "./repositories/event.repository";
import { IEventRepository } from "./types";
import { defaultWebhookDeps, WebhookDeps } from "./controllers/webhook.controller";

type CreateAppOptions = {
  eventRepo?: IEventRepository;
  webhookDeps?: WebhookDeps;
};

export function createApp(options: CreateAppOptions = {}): express.Application {
  const app = express();

  // Create our event storage. Right now it's an in-memory Map,
  // but because it implements the IEventRepository interface,
  // we could swap it for a PostgreSQL-backed version later
  // without changing any other code.
  const eventRepo = options.eventRepo ?? (config.events.storePath
    ? new FileEventRepository(config.events.storePath)
    : new InMemoryEventRepository());
  const webhookDeps = options.webhookDeps ?? defaultWebhookDeps;

  // ── Stripe Webhook Route ──────────────────────────────────
  //
  // IMPORTANT: This route uses express.raw() instead of express.json().
  //
  // Why? Stripe signs the EXACT bytes it sends in the request body.
  // If Express parses the JSON first, and we try to turn it back
  // into a string, the bytes might come out different (different
  // spacing, different key order). That would make the signature
  // check fail, even though the data is valid.
  //
  // By using express.raw(), we keep the body as a raw Buffer
  // (the original bytes), which is exactly what Stripe's SDK
  // needs to verify the signature.
  //
  // We scope this middleware ONLY to the webhook path so the
  // rest of the app can still use normal JSON parsing.
  app.post(
    "/stripe/webhook",
    express.raw({ type: "application/json", limit: config.http.bodyLimit }),
    createWebhookHandler(eventRepo, webhookDeps)
  );

  // For all other routes, parse JSON request bodies normally.
  app.use(express.json({ limit: config.http.bodyLimit }));

  // ── Dashboard API + Real-time Stream ──────────────────────
  // Mounts the /api/events, /api/ledger, and /api/stream routes
  // that power the web dashboard.
  app.use(createDashboardRouter(eventRepo));

  // ── Static Files (Dashboard UI) ───────────────────────────
  // Serve the index.html dashboard from the public folder.
  // We resolve two possible locations so it works whether you
  // run via ts-node (source) or compiled JS (dist).
  const publicDir = path.resolve(__dirname, "..", "src", "public");
  const distPublicDir = path.join(__dirname, "public");
  app.use(express.static(publicDir));
  app.use(express.static(distPublicDir));

  // Simple health check endpoint — useful for monitoring tools
  // to verify the server is alive.
  app.get("/health", (_req, res) => {
    res.json({
      status: "ok",
      stripe_configured: webhookDeps.isStripeConfigured(),
      stripe_expected: isStripeExpected(),
    });
  });

  app.get("/ready", (_req, res) => {
    if (isStripeExpected() && !webhookDeps.isStripeConfigured()) {
      res.status(503).json({
        status: "degraded",
        reason: "stripe-not-configured",
      });
      return;
    }
    res.json({ status: "ready" });
  });

  return app;
}

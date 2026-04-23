// ============================================================
// CONFIG — Loads environment variables and makes them available
// to the rest of the app in a typed, centralized object.
//
// We use a .env file (via the dotenv package) so sensitive keys
// like the Stripe secret never get hardcoded into source code.
// ============================================================

import dotenv from "dotenv";

// Read the .env file from the project root and inject its
// key-value pairs into process.env so we can access them below.
dotenv.config();

// Central config object used throughout the app.
// "as const" makes TypeScript treat these as literal types,
// preventing accidental reassignment.
export const config = {
  // Which port the Express server listens on (defaults to 3001).
  port: parseInt(process.env.PORT || "3001", 10),

  stripe: {
    // The Stripe secret API key (starts with sk_test_ or sk_live_).
    // Used to initialize the Stripe SDK client.
    secretKey: process.env.STRIPE_SECRET_KEY || "",

    // The webhook signing secret (starts with whsec_).
    // Stripe uses this to sign every webhook payload so we can
    // verify the request actually came from Stripe.
    webhookSecret: process.env.STRIPE_WEBHOOK_SECRET || "",
  },

  ledger: {
    // Base URL of the Go ledger engine. Set via LEDGER_ENGINE_URL in docker-compose.
    engineUrl: process.env.LEDGER_ENGINE_URL || "http://ledger-engine:8080",

    // Bearer token for service-to-service calls to the ledger engine.
    serviceToken: process.env.LEDGER_SERVICE_TOKEN || "",

    // Account UUIDs in the ledger DB. Must be created first via POST /accounts.
    // Leave empty to skip DB persistence (dashboard will still show in-memory entries).
    stripeBalanceAccountId: process.env.LEDGER_STRIPE_BALANCE_ACCOUNT_ID || "",
    revenueAccountId: process.env.LEDGER_REVENUE_ACCOUNT_ID || "",
    contraRevenueAccountId: process.env.LEDGER_CONTRA_REVENUE_ACCOUNT_ID || "",
  },
} as const;

// Called at startup to make sure the required env vars exist.
// If either is missing, we crash immediately with a clear error
// rather than failing silently later when a webhook arrives.
export function validateConfig(): void {
  if (!config.stripe.secretKey) {
    throw new Error("STRIPE_SECRET_KEY is required");
  }
  if (!config.stripe.webhookSecret) {
    throw new Error("STRIPE_WEBHOOK_SECRET is required");
  }
}

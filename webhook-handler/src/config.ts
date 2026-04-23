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
  appEnv: (process.env.APP_ENV || "development").toLowerCase(),
  // Which port the Express server listens on (defaults to 3001).
  port: parseInt(process.env.PORT || "3001", 10),
  stripeMode: (process.env.STRIPE_MODE || "optional").toLowerCase(),

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

  events: {
    storePath: process.env.EVENT_STORE_PATH || "",
  },

  http: {
    bodyLimit: process.env.HTTP_BODY_LIMIT || "1mb",
  },
} as const;

function stripeConfigured(): boolean {
  return Boolean(config.stripe.secretKey && config.stripe.webhookSecret);
}

export function validateConfig(): void {
  if (!["production", "staging", "development", "test"].includes(config.appEnv)) {
    throw new Error("APP_ENV must be one of production, staging, development, test");
  }
  if (!["required", "optional", "disabled"].includes(config.stripeMode)) {
    throw new Error("STRIPE_MODE must be one of required, optional, disabled");
  }

  if (config.appEnv === "production" && config.stripeMode === "optional") {
    console.warn("[Config] STRIPE_MODE=optional in production; prefer required or disabled for explicit behavior.");
  }

  if (config.stripeMode === "required" && !stripeConfigured()) {
    throw new Error("Stripe is required but STRIPE_SECRET_KEY / STRIPE_WEBHOOK_SECRET are missing");
  }

  if (config.stripeMode !== "disabled" && !stripeConfigured()) {
    console.warn("[Config] Stripe is not fully configured. Webhook verification will run in degraded mode.");
  }
}

export function isStripeExpected(): boolean {
  return config.stripeMode !== "disabled";
}

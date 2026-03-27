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
  // Which port the Express server listens on (defaults to 3000).
  port: parseInt(process.env.PORT || "3000", 10),

  stripe: {
    // The Stripe secret API key (starts with sk_test_ or sk_live_).
    // Used to initialize the Stripe SDK client.
    secretKey: process.env.STRIPE_SECRET_KEY || "",

    // The webhook signing secret (starts with whsec_).
    // Stripe uses this to sign every webhook payload so we can
    // verify the request actually came from Stripe.
    webhookSecret: process.env.STRIPE_WEBHOOK_SECRET || "",
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

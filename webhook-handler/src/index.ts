// ============================================================
// ENTRY POINT — This is the file that runs when you do
// "npm run dev" or "npm start". It validates configuration,
// creates the Express app, and starts listening for requests.
// ============================================================

import { createApp } from "./app";
import { config, validateConfig } from "./config";

// Make sure all required environment variables are set.
// If anything is missing, the server won't start at all.
validateConfig();

// Build the fully configured Express application
// (routes, middleware, dashboard, etc.)
const app = createApp();

// Start listening for incoming HTTP requests on the configured port.
app.listen(config.port, () => {
  console.log(
    `[Server] Stripe webhook ingestion service listening on port ${config.port}`
  );
});

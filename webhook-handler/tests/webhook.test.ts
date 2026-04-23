import test from "node:test";
import assert from "node:assert/strict";
import http from "node:http";

import { createApp } from "../src/app";
import { InMemoryEventRepository } from "../src/repositories/event.repository";
import type { WebhookDeps } from "../src/controllers/webhook.controller";

type RunningServer = {
  close: () => Promise<void>;
  url: string;
};

async function startServer(deps: Partial<WebhookDeps> = {}): Promise<RunningServer> {
  const repo = new InMemoryEventRepository();
  const app = createApp({
    eventRepo: repo,
    webhookDeps: {
      isStripeConfigured: deps.isStripeConfigured ?? (() => true),
      verifyWebhookSignature: deps.verifyWebhookSignature ?? (((() => ({
        id: "evt_test_1",
        type: "charge.succeeded",
        created: Math.floor(Date.now() / 1000),
        data: {
          object: {
            id: "ch_123",
            amount: 5000,
            currency: "usd",
            customer: "cus_123",
          },
        },
      })) as unknown) as WebhookDeps["verifyWebhookSignature"]),
      postToLedger: deps.postToLedger ?? (async () => undefined),
    },
  });

  const server = http.createServer(app);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", () => resolve()));
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("failed to start test server");
  }

  return {
    url: `http://127.0.0.1:${address.port}`,
    close: () => new Promise<void>((resolve, reject) => server.close((err) => err ? reject(err) : resolve())),
  };
}

test("webhook processes a valid event and blocks duplicates", async () => {
  let postCalls = 0;
  const running = await startServer({
    postToLedger: async () => {
      postCalls += 1;
    },
  });

  try {
    const first = await fetch(`${running.url}/stripe/webhook`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "stripe-signature": "sig_test",
      },
      body: JSON.stringify({ hello: "world" }),
    });
    assert.equal(first.status, 200);
    assert.deepEqual(await first.json(), { received: true, processed: true });

    const second = await fetch(`${running.url}/stripe/webhook`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "stripe-signature": "sig_test",
      },
      body: JSON.stringify({ hello: "world" }),
    });
    assert.equal(second.status, 200);
    assert.deepEqual(await second.json(), { received: true, duplicate: true });
    assert.equal(postCalls, 1);
  } finally {
    await running.close();
  }
});

test("webhook rejects invalid signatures safely", async () => {
  const running = await startServer({
    verifyWebhookSignature: (() => {
      throw new Error("bad signature");
    }) as WebhookDeps["verifyWebhookSignature"],
  });

  try {
    const response = await fetch(`${running.url}/stripe/webhook`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "stripe-signature": "sig_bad",
      },
      body: JSON.stringify({ hello: "world" }),
    });

    assert.equal(response.status, 400);
    assert.deepEqual(await response.json(), { error: "Invalid signature" });
  } finally {
    await running.close();
  }
});

test("webhook enters explicit degraded mode when Stripe is unavailable", async () => {
  const running = await startServer({
    isStripeConfigured: () => false,
  });

  try {
    const response = await fetch(`${running.url}/stripe/webhook`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify({ hello: "world" }),
    });

    assert.equal(response.status, 503);
    assert.deepEqual(await response.json(), {
      error: "Stripe webhook handling is disabled because Stripe is not configured",
    });
  } finally {
    await running.close();
  }
});

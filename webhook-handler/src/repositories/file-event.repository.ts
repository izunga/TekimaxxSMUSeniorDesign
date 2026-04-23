import fs from "fs/promises";
import path from "path";
import Stripe from "stripe";

import { IEventRepository, StoredEvent } from "../types";

type PersistedStoredEvent = Omit<StoredEvent, "receivedAt" | "processedAt"> & {
  receivedAt: string;
  processedAt: string | null;
};

export class FileEventRepository implements IEventRepository {
  private readonly store = new Map<string, StoredEvent>();
  private readonly filePath: string;
  private readonly ready: Promise<void>;

  constructor(filePath: string) {
    this.filePath = filePath;
    this.ready = this.load();
  }

  async saveRawEvent(event: Stripe.Event): Promise<void> {
    await this.ready;
    this.store.set(event.id, {
      eventId: event.id,
      status: "RECEIVED",
      rawEvent: event,
      receivedAt: new Date(),
      processedAt: null,
      failureReason: null,
    });
    await this.flush();
  }

  async hasSeenEvent(eventId: string): Promise<boolean> {
    await this.ready;
    return this.store.has(eventId);
  }

  async markEventPosted(eventId: string): Promise<void> {
    await this.ready;
    const entry = this.store.get(eventId);
    if (!entry) {
      return;
    }
    entry.status = "POSTED";
    entry.processedAt = new Date();
    await this.flush();
  }

  async markEventFailed(eventId: string, reason: string): Promise<void> {
    await this.ready;
    const entry = this.store.get(eventId);
    if (!entry) {
      return;
    }
    entry.status = "FAILED";
    entry.processedAt = new Date();
    entry.failureReason = reason;
    await this.flush();
  }

  async markEventIgnored(eventId: string): Promise<void> {
    await this.ready;
    const entry = this.store.get(eventId);
    if (!entry) {
      return;
    }
    entry.status = "IGNORED";
    entry.processedAt = new Date();
    await this.flush();
  }

  async getEvent(eventId: string): Promise<StoredEvent | undefined> {
    await this.ready;
    return this.store.get(eventId);
  }

  async getAllEvents(): Promise<StoredEvent[]> {
    await this.ready;
    return Array.from(this.store.values()).sort(
      (a, b) => b.receivedAt.getTime() - a.receivedAt.getTime()
    );
  }

  private async load(): Promise<void> {
    await fs.mkdir(path.dirname(this.filePath), { recursive: true });
    try {
      const raw = await fs.readFile(this.filePath, "utf-8");
      const parsed = JSON.parse(raw) as PersistedStoredEvent[];
      for (const item of parsed) {
        this.store.set(item.eventId, {
          ...item,
          rawEvent: item.rawEvent as Stripe.Event,
          receivedAt: new Date(item.receivedAt),
          processedAt: item.processedAt ? new Date(item.processedAt) : null,
        });
      }
    } catch (error) {
      const err = error as NodeJS.ErrnoException;
      if (err.code !== "ENOENT") {
        throw error;
      }
    }
  }

  private async flush(): Promise<void> {
    const snapshot: PersistedStoredEvent[] = Array.from(this.store.values()).map((entry) => ({
      ...entry,
      receivedAt: entry.receivedAt.toISOString(),
      processedAt: entry.processedAt ? entry.processedAt.toISOString() : null,
    }));

    const tmp = `${this.filePath}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(snapshot, null, 2), "utf-8");
    await fs.rename(tmp, this.filePath);
  }
}

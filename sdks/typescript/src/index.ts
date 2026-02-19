/**
 * eventrelay - TypeScript SDK for sending events to an eventrelay server.
 *
 * @example
 * ```ts
 * import { Client } from "eventrelay";
 *
 * const er = new Client("http://localhost:6060/events", "myapp");
 * er.emit("deploy", { env: "prod" });
 * er.error("crash", { msg: "something broke" });
 *
 * // Timed operations
 * const done = er.timed("db_query");
 * await doQuery();
 * done({ rows: 42 });
 *
 * await er.flush();
 * ```
 */

export interface Event {
  source?: string;
  channel?: string;
  action?: string;
  level?: string;
  agent_id?: string;
  duration_ms?: number;
  data?: Record<string, unknown>;
  ts?: string;
}

export class Client {
  private url: string;
  private source: string;
  private channel: string;
  private pending: Promise<void>[] = [];

  constructor(url: string = "", source: string = "", channel: string = "") {
    this.url = url;
    this.source = source;
    this.channel = channel;
  }

  /** Return a new Client that tags all events with the given channel. */
  withChannel(channel: string): Client {
    return new Client(this.url, this.source, channel);
  }

  /** Send an info-level event. */
  emit(action: string, data?: Record<string, unknown>): void {
    this.send("info", action, data);
  }

  /** Send an error-level event. */
  error(action: string, data?: Record<string, unknown>): void {
    this.send("error", action, data);
  }

  /** Send a warn-level event. */
  warn(action: string, data?: Record<string, unknown>): void {
    this.send("warn", action, data);
  }

  /** Send a debug-level event. */
  debug(action: string, data?: Record<string, unknown>): void {
    this.send("debug", action, data);
  }

  /** Send a fully customized event. */
  emitEvent(event: Event): void {
    if (!this.url) return;
    const evt = {
      source: event.source || this.source,
      channel: event.channel || this.channel,
      level: event.level || "info",
      ts: event.ts || new Date().toISOString(),
      ...event,
    };
    // Re-apply defaults after spread in case event had empty strings
    if (!evt.source) evt.source = this.source;
    if (!evt.channel) evt.channel = this.channel;
    this.post(evt);
  }

  /**
   * Start timing an operation. Returns a function that emits the event
   * with duration_ms set when called.
   *
   * @example
   * ```ts
   * const done = er.timed("db_query");
   * const result = await doQuery();
   * done({ rows: result.length });
   * ```
   */
  timed(action: string, data?: Record<string, unknown>): (extra?: Record<string, unknown>) => void {
    const start = performance.now();
    return (extra?: Record<string, unknown>) => {
      const duration_ms = Math.round(performance.now() - start);
      this.emitEvent({
        action,
        level: "info",
        duration_ms,
        data: { ...data, ...extra },
      });
    };
  }

  /** Wait for all pending events to be sent. */
  async flush(): Promise<void> {
    await Promise.allSettled(this.pending);
    this.pending = [];
  }

  private send(level: string, action: string, data?: Record<string, unknown>): void {
    if (!this.url) return;
    const evt: Event = {
      source: this.source,
      channel: this.channel || undefined,
      action,
      level,
      data,
      ts: new Date().toISOString(),
    };
    this.post(evt);
  }

  private post(payload: Record<string, unknown>): void {
    // Remove empty string values
    const clean = Object.fromEntries(
      Object.entries(payload).filter(([, v]) => v !== "" && v !== undefined)
    );
    const p = fetch(this.url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(clean),
    })
      .then(() => {})
      .catch(() => {}); // fire and forget
    this.pending.push(p);
  }
}

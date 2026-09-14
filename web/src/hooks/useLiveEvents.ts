import { useEffect, useState } from "react";

export type LiveState =
  "idle" | "connecting" | "connected" | "reconnecting" | "unauthorized";

function pause(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const done = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", done);
      resolve();
    };
    const timer = setTimeout(done, ms);
    signal.addEventListener("abort", done, { once: true });
    if (signal.aborted) done();
  });
}

// Fetch-based SSE keeps bearer credentials out of URLs. Every connection begins
// with a sync event; dropped/coalesced events are recovered by a fresh API read.
export function useLiveEvents(path: string | undefined, token: string | null) {
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<LiveState>("idle");
  useEffect(() => {
    if (!path) {
      setState("idle");
      return;
    }
    const controller = new AbortController();
    const { signal } = controller;
    const resync = () => {
      if (!document.hidden) setRevision((n) => n + 1);
    };
    document.addEventListener("visibilitychange", resync);
    let attempts = 0;
    async function run() {
      while (!signal.aborted) {
        setState(attempts ? "reconnecting" : "connecting");
        let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
        try {
          const response = await fetch(path!, {
            signal,
            headers: token ? { Authorization: `Bearer ${token}` } : {},
          });
          if (response.status === 401 || response.status === 403) {
            setState("unauthorized");
            return;
          }
          if (!response.ok || !response.body)
            throw new Error("Event stream unavailable");
          reader = response.body.getReader();
          const decoder = new TextDecoder();
          let pending = "";
          setState("connected");
          attempts = 0;
          while (!signal.aborted) {
            const { value, done } = await reader.read();
            if (done) break;
            pending += decoder.decode(value, { stream: true });
            let boundary: number;
            while ((boundary = pending.indexOf("\n\n")) >= 0) {
              const frame = pending.slice(0, boundary);
              pending = pending.slice(boundary + 2);
              if (frame.includes("event: revoked")) {
                setState("unauthorized");
                return;
              }
              if (frame.includes("event: sync")) setRevision((n) => n + 1);
            }
          }
        } catch {
          /* Retry transient network failures; never keep stale status. */
        } finally {
          await reader?.cancel().catch(() => {});
        }
        if (signal.aborted) return;
        setState("reconnecting");
        await pause(Math.min(1000 * 2 ** attempts++, 15000), signal);
      }
    }
    void run();
    return () => {
      controller.abort();
      document.removeEventListener("visibilitychange", resync);
    };
  }, [path, token]);
  return { revision, state };
}

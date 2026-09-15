import { useEffect, useState } from "react";
import { api } from "../api";

interface Event {
  id: number;
  status: string;
  at: string;
  detail: string;
}
interface Attempt {
  headers?: Record<string, string>;
  response_truncated?: boolean;
  id: number;
  job_id: string;
  at: string;
  url: string;
  request: string;
  content_type: string;
  response: string;
  status: number;
  error: string;
  duration_ms: number;
}
export function MessageEvents({
  id,
  revision,
  operator = true,
}: {
  id: string;
  revision: number;
  operator?: boolean;
}) {
  const [events, setEvents] = useState<Event[]>([]);
  const [attempts, setAttempts] = useState<Attempt[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([
      api<{ events: Event[] }>(`/messages/${id}/events`, {
        signal: controller.signal,
      }),
      operator
        ? api<{ attempts: Attempt[] }>(
            `/webhooks?message_id=${encodeURIComponent(id)}`,
            { signal: controller.signal },
          )
        : Promise.resolve({ attempts: [] as Attempt[] }),
    ])
      .then(([history, deliveries]) => {
        setEvents(history.events);
        setAttempts(deliveries.attempts);
        setError("");
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, [id, revision, refresh, operator]);
  async function retry(job: string) {
    try {
      await api(`/webhooks/${job}/retry`, { method: "POST" });
      setNotice("Callback replay scheduled.");
      setRefresh((n) => n + 1);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <div className="event-view">
      <h3>Message history</h3>
      <ol className="event-list">
        {events.map((event) => (
          <li key={event.id}>
            <time>{new Date(event.at).toLocaleTimeString()}</time>
            <strong>{event.status}</strong>
            <span>{event.detail}</span>
          </li>
        ))}
      </ol>
      {operator && (
        <>
          <h3>Webhook attempts</h3>
          {attempts.length ? (
            attempts.map((attempt) => (
              <details className="webhook-attempt" key={attempt.id}>
                <summary>
                  <span>{attempt.status || "Network error"}</span> ·{" "}
                  {attempt.duration_ms}ms ·{" "}
                  {new Date(attempt.at).toLocaleTimeString()}
                </summary>
                <p className="webhook-url">{attempt.url}</p>
                <small>{attempt.content_type}</small>
                <pre>{JSON.stringify(attempt.headers || {}, null, 2)}</pre>
                <pre>{attempt.request}</pre>
                <h4>Response</h4>
                <pre>{attempt.error || attempt.response || "(empty body)"}</pre>
                {attempt.response_truncated && (
                  <p className="modal-description">
                    Response preview limited to 4096 bytes.
                  </p>
                )}
                <button
                  className="secondary"
                  onClick={() => {
                    void retry(attempt.job_id);
                  }}
                >
                  Replay callback
                </button>
              </details>
            ))
          ) : (
            <p className="modal-description">
              No callback attempts recorded for this message.
            </p>
          )}
        </>
      )}
      {notice && (
        <p role="status" className="modal-description">
          {notice}
        </p>
      )}
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
    </div>
  );
}

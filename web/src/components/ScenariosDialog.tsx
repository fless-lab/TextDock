import { useState, type FormEvent } from "react";
import { Trash2 } from "lucide-react";
import { api } from "../api";
import { Modal } from "./Modal";

export interface Scenario {
  id: string;
  inbox: string;
  name: string;
  prefix: string;
  outcome: string;
  delay_ms: number;
  failure_percent: number;
  seed: string;
  reject_status: number;
  retry_after: number;
  webhook_url: string;
  webhook_format: string;
}
export function ScenariosDialog({
  inbox,
  scenarios,
  changed,
  close,
}: {
  inbox: string;
  scenarios: Scenario[];
  changed: () => void;
  close: () => void;
}) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api("/scenarios", {
        method: "POST",
        body: JSON.stringify({
          ...data,
          inbox,
          delay_ms: Number(data.delay_ms),
          failure_percent: Number(data.failure_percent),
          reject_status: Number(data.reject_status),
          retry_after: Number(data.retry_after),
        }),
      });
      changed();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(id: string) {
    try {
      await api(`/scenarios/${id}`, { method: "DELETE" });
      changed();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <Modal title="Simulation scenarios" close={close}>
      <p className="modal-description">
        Scenarios belong to this inbox. Select one when sending a simulated
        message; capture mode remains the default.
      </p>
      <ul className="device-list">
        {scenarios.map((s) => (
          <li key={s.id}>
            <div>
              <strong>{s.name}</strong>
              <small>
                {s.reject_status
                  ? `HTTP ${s.reject_status}`
                  : `${s.outcome} after ${s.delay_ms}ms`}{" "}
                · {s.prefix || "all recipients"}
              </small>
            </div>
            <button
              className="icon-button"
              aria-label={`Delete scenario ${s.name}`}
              onClick={() => {
                void remove(s.id);
              }}
            >
              <Trash2 size={15} />
            </button>
          </li>
        ))}
      </ul>
      <h3>New scenario</h3>
      <form
        onSubmit={(e) => {
          void create(e);
        }}
      >
        <label>
          Scenario name
          <input
            name="name"
            placeholder="e.g. Delayed delivery"
            maxLength={80}
            required
          />
        </label>
        <div className="form-row">
          <label>
            Recipient prefix
            <input
              name="prefix"
              placeholder="e.g. +33 (optional)"
              maxLength={16}
            />
          </label>
          <label>
            Outcome
            <select name="outcome">
              <option value="delivered">Delivered</option>
              <option value="failed">Failed</option>
              <option value="expired">Expired</option>
            </select>
          </label>
        </div>
        <div className="form-row">
          <label>
            Delay (ms)
            <input
              name="delay_ms"
              type="number"
              min={0}
              max={86400000}
              defaultValue={500}
              required
            />
          </label>
          <label>
            Failure percentage
            <input
              name="failure_percent"
              type="number"
              min={0}
              max={100}
              defaultValue={0}
              required
            />
          </label>
        </div>
        <div className="form-row">
          <label>
            Seed
            <input name="seed" defaultValue="0" maxLength={128} />
          </label>
          <label>
            Reject request
            <select name="reject_status">
              <option value="0">No rejection</option>
              <option value="400">400 · Invalid request</option>
              <option value="429">429 · Rate limited</option>
              <option value="503">503 · Unavailable</option>
            </select>
          </label>
        </div>
        <label>
          Retry-After (seconds)
          <input
            name="retry_after"
            type="number"
            min={0}
            max={3600}
            defaultValue={5}
          />
        </label>
        <label>
          Delivery webhook URL
          <input
            name="webhook_url"
            type="url"
            placeholder="http://localhost:3000/webhooks/sms (optional)"
          />
        </label>
        <label>
          Webhook format
          <select name="webhook_format">
            <option value="json">TextDock JSON</option>
            <option value="twilio">Twilio status form</option>
          </select>
        </label>
        <p className="modal-description">
          Set TEXTDOCK_WEBHOOK_SECRET on the server to sign callbacks. Failed
          callbacks are retried up to five times and can be replayed from
          message events.
        </p>
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "Saving…" : "Create scenario"}
        </button>
      </form>
    </Modal>
  );
}

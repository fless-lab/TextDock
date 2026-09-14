import { useEffect, useRef, useState, type FormEvent } from "react";
import { RefreshCw } from "lucide-react";
import { api, type Message } from "../api";
import { Modal } from "./Modal";

interface Device {
  serial: string;
  state: string;
  model: string;
  kind: string;
  can_inject: boolean;
  reason: string;
}
interface LabStatus {
  enabled: boolean;
  available: boolean;
  version: string;
  error?: string;
  devices: Device[];
}
interface Injection {
  id: string;
  message_id: string;
  serial: string;
  status: string;
  detail: string;
  output: string;
  created_at: string;
}

export function DeviceLabDialog({
  inbox,
  close,
  changed,
}: {
  inbox: string;
  close: () => void;
  changed: () => void;
}) {
  const [status, setStatus] = useState<LabStatus>();
  const [history, setHistory] = useState<Injection[]>([]);
  const [serial, setSerial] = useState("");
  const [busy, setBusy] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [result, setResult] = useState<Injection>();
  const key = useRef(
    Array.from(crypto.getRandomValues(new Uint32Array(4))).join("-"),
  );
  const abort = useRef(new AbortController());

  async function load() {
    const signal = abort.current.signal;
    setRefreshing(true);
    try {
      const [current, log] = await Promise.all([
        api<LabStatus>("/lab/status", { signal }),
        api<{ injections: Injection[] }>(
          `/lab/history?inbox=${encodeURIComponent(inbox)}`,
          { signal },
        ),
      ]);
      if (signal.aborted) return;
      setStatus(current);
      setHistory(log.injections);
      setError("");
      setSerial((selected) =>
        current.devices.some((d) => d.serial === selected && d.can_inject)
          ? selected
          : "",
      );
    } catch (e) {
      if (!signal.aborted) setError((e as Error).message);
    } finally {
      if (!signal.aborted) setRefreshing(false);
    }
  }
  useEffect(() => {
    abort.current = new AbortController();
    void load();
    return () => abort.current.abort();
  }, [inbox]);

  async function inject(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const response = await api<{
        message: Message;
        injection: Injection;
        replayed: boolean;
      }>("/lab/injections", {
        method: "POST",
        signal: abort.current.signal,
        body: JSON.stringify({
          ...Object.fromEntries(new FormData(e.currentTarget)),
          inbox,
          serial,
          idempotency_key: key.current,
        }),
      });
      setResult(response.injection);
      setNotice(
        response.injection.status === "injected"
          ? `SMS accepted by ${serial}. No carrier SMS sent.`
          : `Injection ${response.injection.status}. Inspect the console result before creating another attempt.`,
      );
      changed();
      await load();
    } catch (e) {
      if (!abort.current.signal.aborted) setError((e as Error).message);
    } finally {
      if (!abort.current.signal.aborted) setBusy(false);
    }
  }
  return (
    <Modal title="Android device lab" close={close}>
      <p className="modal-description">
        Inject a simulated incoming SMS into a selected Android emulator. ADB
        runs on the TextDock server; physical phone SIMs are not used.
      </p>
      <div className="devices-heading">
        <h3>ADB connection</h3>
        <button
          className="icon-button"
          aria-label="Refresh device lab"
          disabled={busy || refreshing}
          onClick={() => {
            void load();
          }}
        >
          <RefreshCw size={16} />
        </button>
      </div>
      {!status ? (
        <p className="modal-description">Checking ADB…</p>
      ) : !status.enabled ? (
        <p className="callout">
          Enable the lab with <code>TEXTDOCK_ADB_ENABLED=true</code>. Install
          Android Platform Tools on the server; optionally set{" "}
          <code>TEXTDOCK_ADB_PATH</code> to the executable.
        </p>
      ) : (
        <>
          {status.error && (
            <p role="alert" className="error-text">
              {status.error}
            </p>
          )}
          {status.version && <pre className="code-block">{status.version}</pre>}
          {!status.devices.length && status.available && (
            <p className="modal-description">
              No devices found. Start an Android Virtual Device, then refresh.
            </p>
          )}
          <ul className="device-list">
            {status.devices.map((d) => (
              <li key={d.serial}>
                <div>
                  <strong>{d.serial}</strong>
                  <small>
                    {d.model || d.kind} · {d.state}
                    {d.reason ? ` · ${d.reason}` : ""}
                  </small>
                </div>
                <span className="muted">
                  {d.can_inject ? "Emulator" : "Unavailable for injection"}
                </span>
              </li>
            ))}
          </ul>
          <h3>Inject a test SMS</h3>
          <form
            onSubmit={(e) => {
              void inject(e);
            }}
          >
            <label>
              Target emulator
              <select
                value={serial}
                required
                disabled={busy}
                onChange={(e) => {
                  setSerial(e.target.value);
                  setResult(undefined);
                  key.current = Array.from(
                    crypto.getRandomValues(new Uint32Array(4)),
                  ).join("-");
                }}
              >
                <option value="" disabled>
                  Select an online emulator
                </option>
                {status.devices
                  .filter((d) => d.can_inject)
                  .map((d) => (
                    <option key={d.serial} value={d.serial}>
                      {d.serial} · {d.model || "Android emulator"}
                    </option>
                  ))}
              </select>
            </label>
            <div className="form-row">
              <label>
                SMS sender
                <input
                  name="from"
                  type="tel"
                  defaultValue="+12025550100"
                  required
                />
              </label>
              <label>
                TextDock test recipient
                <input
                  name="to"
                  type="tel"
                  defaultValue="+12025550123"
                  required
                />
              </label>
            </div>
            <p className="modal-description">
              The serial chooses the emulator. The test recipient only scopes
              the message in your TextDock inbox.
            </p>
            <label>
              SMS text
              <textarea
                name="body"
                rows={4}
                defaultValue={
                  "Your code is 482193.\n\n@login.example.test #482193"
                }
                required
              />
            </label>
            <label>
              Test run ID
              <input
                name="run_id"
                placeholder="e.g. emulator-login-42"
                maxLength={128}
              />
            </label>
            <div className="modal-footer">
              <button
                type="button"
                className="secondary"
                disabled={busy}
                onClick={() => {
                  key.current = Array.from(
                    crypto.getRandomValues(new Uint32Array(4)),
                  ).join("-");
                  setResult(undefined);
                  setNotice("New injection intent.");
                }}
              >
                New attempt
              </button>
              <button
                className="primary"
                disabled={!serial || busy || !!result}
              >
                {busy ? "Injecting…" : "Inject into emulator"}
              </button>
            </div>
          </form>
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
      {result && (
        <details open>
          <summary>Console result · {result.status}</summary>
          <pre className="code-block">{result.output || result.detail}</pre>
        </details>
      )}
      <h3>Injection history</h3>
      <p className="modal-description">
        Latest attempts in this inbox. “Injected” confirms emulator-console
        acceptance, not carrier delivery or native OTP autofill.
      </p>
      {!history.length ? (
        <p className="modal-description">No injection attempts yet.</p>
      ) : (
        <ol className="lab-history">
          {history.map((item) => (
            <li key={item.id}>
              <strong>
                {item.serial} · {item.status}
              </strong>
              <time>{new Date(item.created_at).toLocaleString()}</time>
              <span>{item.detail}</span>
              <code>{item.message_id}</code>
            </li>
          ))}
        </ol>
      )}
    </Modal>
  );
}

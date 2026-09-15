import { useEffect, useRef, useState } from "react";
import { api, type UserSession } from "../api";
import { Modal } from "./Modal";

export function AccountDialog({
  session,
  close,
  ended,
}: {
  session: UserSession;
  close: () => void;
  ended: () => void;
}) {
  const [sessions, setSessions] = useState<
    { id: string; created_at: string; expires_at: string }[]
  >([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [revision, setRevision] = useState(0);
  const active = useRef(true);
  useEffect(() => {
    active.current = true;
    return () => {
      active.current = false;
    };
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    void api<{ sessions: typeof sessions }>("/account/sessions", {
      signal: controller.signal,
    })
      .then((v) => setSessions(v.sessions))
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, [revision]);
  async function change(action: () => Promise<unknown>, end = false) {
    setBusy(true);
    setError("");
    try {
      await action();
      if (!active.current) return;
      if (end) ended();
      else setRevision((n) => n + 1);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="Your account" close={close}>
      <p>
        {session.user.name} · {session.user.username}
      </p>
      <button
        className="secondary"
        disabled={busy}
        onClick={() => {
          void change(() => api("/account/logout", { method: "POST" }), true);
        }}
      >
        Sign out
      </button>
      <h3>Change password</h3>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          const input = Object.fromEntries(new FormData(e.currentTarget));
          void change(
            () =>
              api("/account/password", {
                method: "PUT",
                body: JSON.stringify(input),
              }),
            true,
          );
        }}
      >
        <label>
          Current password
          <input
            type="password"
            name="current_password"
            minLength={12}
            maxLength={128}
            required
            autoComplete="current-password"
          />
        </label>
        <label>
          New password
          <input
            type="password"
            name="password"
            minLength={12}
            maxLength={128}
            required
            autoComplete="new-password"
          />
        </label>
        <button className="primary" disabled={busy}>
          Change password and sign out
        </button>
      </form>
      <h3>Active sessions</h3>
      <ul className="key-list">
        {sessions.map((s) => (
          <li key={s.id}>
            <strong>
              {s.id === session.id ? "This session" : "Another session"}
            </strong>
            <p>
              Started {new Date(s.created_at).toLocaleString()} · Expires{" "}
              {new Date(s.expires_at).toLocaleString()}
            </p>
            <button
              className="secondary"
              disabled={busy}
              aria-label={`End session ${s.id}`}
              onClick={() => {
                void change(
                  () => api(`/account/sessions/${s.id}`, { method: "DELETE" }),
                  s.id === session.id,
                );
              }}
            >
              End session
            </button>
          </li>
        ))}
      </ul>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
    </Modal>
  );
}

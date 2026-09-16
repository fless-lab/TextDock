import { useEffect, useState } from "react";
import { api } from "../api";
import { Modal } from "./Modal";

interface AuditEvent {
  id: number;
  at: string;
  actor: { kind: string; id?: string };
  action: string;
  outcome: string;
  project_id?: string;
  inbox_id?: string;
  resource_type: string;
  resource_id?: string;
  count: number;
  role_before?: string;
  role_after?: string;
  permissions?: string[];
}
export function AuditDialog({
  projects,
  operator,
  close,
}: {
  projects: { id: string; name: string }[];
  operator: boolean;
  close: () => void;
}) {
  const [project, setProject] = useState(operator ? "" : projects[0]?.id || "");
  const [action, setAction] = useState("");
  const [outcome, setOutcome] = useState("");
  const [actor, setActor] = useState("");
  const [actions, setActions] = useState<string[]>([]);
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [cursor, setCursor] = useState(0);
  const [next, setNext] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  function reset() {
    setCursor(0);
    setEvents([]);
    setNext(0);
  }
  useEffect(() => {
    const controller = new AbortController();
    setBusy(true);
    setError("");
    const timer = setTimeout(() => {
      const query = new URLSearchParams({ limit: "50" });
      if (project) query.set("project_id", project);
      if (action) query.set("action", action);
      if (outcome) query.set("outcome", outcome);
      if (actor) query.set("actor_id", actor);
      if (cursor) query.set("before", String(cursor));
      void api<{
        events: AuditEvent[];
        actions: string[];
        next_cursor: number;
      }>(`/audit?${query}`, { signal: controller.signal })
        .then((data) => {
          if (!controller.signal.aborted) {
            setEvents(data.events);
            setActions(data.actions);
            setNext(data.next_cursor);
          }
        })
        .catch((e) => {
          if (!controller.signal.aborted) {
            setError(e.message);
            setEvents([]);
            setNext(0);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setBusy(false);
        });
    }, 150);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [project, action, outcome, actor, cursor, revision]);
  function download() {
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(events, null, 2)], { type: "application/json" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "textdock-audit.json";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  return (
    <Modal title="Audit log" close={close}>
      <p className="modal-description">
        Committed resource changes, verified login failures and prepared
        exports. Metadata only: no SMS, OTP, phone number, password or token. An
        export event does not prove the client received the download.
      </p>
      <div className="form-row">
        <label>
          Audit project
          <select
            value={project}
            onChange={(e) => {
              setProject(e.target.value);
              reset();
            }}
          >
            {operator && (
              <option value="">All projects and instance events</option>
            )}
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Audit action
          <select
            value={action}
            onChange={(e) => {
              setAction(e.target.value);
              reset();
            }}
          >
            <option value="">All actions</option>
            {actions.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="form-row">
        <label>
          Outcome
          <select
            value={outcome}
            onChange={(e) => {
              setOutcome(e.target.value);
              reset();
            }}
          >
            <option value="">All outcomes</option>
            <option value="committed">Committed</option>
            <option value="denied">Denied</option>
            <option value="prepared">Prepared</option>
          </select>
        </label>
        <label>
          Actor ID
          <input
            value={actor}
            maxLength={128}
            onChange={(e) => {
              setActor(e.target.value);
              reset();
            }}
            placeholder="User or API key ID"
          />
        </label>
      </div>
      <div className="form-row">
        <button
          className="secondary"
          disabled={busy}
          onClick={() => {
            reset();
            setRevision((n) => n + 1);
          }}
        >
          Newest events
        </button>
        <button
          className="secondary"
          disabled={busy || !events.length}
          onClick={download}
        >
          Download audit page
        </button>
        <button
          className="secondary"
          disabled={busy || !next}
          onClick={() => {
            setCursor(next);
            setEvents([]);
          }}
        >
          Older events
        </button>
      </div>
      {busy && (
        <p role="status" className="modal-description">
          Loading audit events…
        </p>
      )}
      {!busy && !error && !events.length && (
        <p className="modal-description">No matching audit events.</p>
      )}
      <ol className="key-list audit-list">
        {events.map((e) => (
          <li key={e.id} data-audit-action={e.action}>
            <strong>{e.action}</strong> · {e.outcome}
            <p>
              <time dateTime={e.at}>{new Date(e.at).toLocaleString()}</time> · #
              {e.id}
            </p>
            <p>
              Actor: {e.actor.kind}
              {e.actor.id && (
                <>
                  {" "}
                  · <code>{e.actor.id}</code>
                </>
              )}
            </p>
            <p>
              {e.project_id
                ? projects.find((p) => p.id === e.project_id)?.name ||
                  e.project_id
                : "Instance"}
              {e.inbox_id && (
                <>
                  {" "}
                  · <code>{e.inbox_id}</code>
                </>
              )}
            </p>
            <p>
              {e.resource_type}
              {e.resource_id && (
                <>
                  {" "}
                  · <code>{e.resource_id}</code>
                </>
              )}{" "}
              · {e.count} affected
            </p>
            {(e.role_before || e.role_after) && (
              <p>
                Role: {e.role_before || "none"} → {e.role_after || "none"}
              </p>
            )}
            {!!e.permissions?.length && (
              <p>Permissions: {e.permissions.join(", ")}</p>
            )}
          </li>
        ))}
      </ol>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
    </Modal>
  );
}

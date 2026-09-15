import { useEffect, useState, type FormEvent } from "react";
import { api } from "../api";
import { copyText } from "../clipboard";
import { Modal } from "./Modal";
import type { Workspaces } from "./WorkspacesDialog";

interface APIKey {
  id: string;
  name: string;
  project_id: string;
  inbox_id?: string;
  permissions: string[];
  expires_at: string;
  revoked_at?: string;
}
export function KeysDialog({
  data,
  inbox,
  authEnabled,
  close,
}: {
  data: Workspaces;
  inbox: string;
  authEnabled: boolean;
  close: () => void;
}) {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [project, setProject] = useState(
    data.inboxes.find((b) => b.id === inbox)?.project_id ||
      data.projects[0]?.id ||
      "default",
  );
  const [scope, setScope] = useState("inbox");
  const [box, setBox] = useState(inbox);
  const [secret, setSecret] = useState<{ token: string; id: string }>();
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function refresh() {
    setKeys((await api<{ keys: APIKey[] }>("/keys")).keys);
  }
  useEffect(() => {
    void refresh().catch((e) => setError((e as Error).message));
  }, []);
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setSecret(undefined);
    setCopied(false);
    const form = new FormData(event.currentTarget);
    try {
      const result = await api<{ key: APIKey; token: string }>("/keys", {
        method: "POST",
        body: JSON.stringify({
          name: form.get("name"),
          project_id: project,
          inbox_id: scope === "inbox" ? box : "",
          permissions: form.getAll("permission"),
          expires_at: new Date(
            Date.now() + Number(form.get("days")) * 86400000,
          ).toISOString(),
        }),
      });
      setSecret({ token: result.token, id: result.key.id });
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    setBusy(true);
    setError("");
    try {
      await api(`/keys/${id}`, { method: "DELETE" });
      if (secret?.id === id) setSecret(undefined);
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="API keys" close={close}>
      <p className="modal-description">
        Give a CI job or integration access to one project or inbox. Keys
        support local capture, scoped reads and deletion. Relay, simulation and
        administration require the operator token.
      </p>
      {!authEnabled && (
        <p className="modal-description">
          This server permits anonymous local operator access. Configure
          TEXTDOCK_TOKEN to enforce access control when sharing the instance.
        </p>
      )}
      <form
        onSubmit={(e) => {
          void create(e);
        }}
      >
        <label>
          Key name
          <input
            name="name"
            required
            maxLength={80}
            placeholder="Checkout CI"
          />
        </label>
        <div className="form-row">
          <label>
            Project
            <select
              value={project}
              onChange={(e) => {
                setProject(e.target.value);
                setBox(
                  data.inboxes.find((b) => b.project_id === e.target.value)
                    ?.id || "",
                );
              }}
            >
              {data.projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Scope
            <select value={scope} onChange={(e) => setScope(e.target.value)}>
              <option value="inbox">One inbox</option>
              <option value="project">Entire project</option>
            </select>
          </label>
        </div>
        {scope === "inbox" && (
          <label>
            Allowed inbox
            <select value={box} onChange={(e) => setBox(e.target.value)}>
              {data.inboxes
                .filter((b) => b.project_id === project)
                .map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
            </select>
          </label>
        )}
        <fieldset className="key-permissions">
          <legend>Permissions</legend>
          <label>
            <input
              type="checkbox"
              name="permission"
              value="messages:read"
              defaultChecked
            />
            Read messages, OTP, events and exports
          </label>
          <label>
            <input
              type="checkbox"
              name="permission"
              value="messages:write"
              defaultChecked
            />
            Capture messages and edit metadata (editing also needs read)
          </label>
          <label>
            <input type="checkbox" name="permission" value="messages:delete" />
            Delete messages and purge inboxes
          </label>
        </fieldset>
        <label>
          Expires in days
          <input
            type="number"
            name="days"
            min={1}
            max={365}
            defaultValue={30}
            required
          />
        </label>
        <button className="primary" disabled={busy}>
          Create API key
        </button>
      </form>
      {secret && (
        <section className="key-secret">
          <h3>Copy this key now</h3>
          <p className="modal-description">
            It is shown only here and will disappear when you close this dialog.
            The server stores its hash.
          </p>
          <label>
            New API key
            <textarea readOnly value={secret.token} spellCheck={false} />
          </label>
          <button
            className="secondary"
            onClick={async () => setCopied(await copyText(secret.token))}
          >
            {copied ? "Copied" : "Copy API key"}
          </button>
        </section>
      )}
      <h3>Existing keys</h3>
      {keys.length === 0 && (
        <p className="modal-description">No API keys created.</p>
      )}
      <ul className="key-list">
        {keys.map((k) => (
          <li key={k.id}>
            <strong>{k.name}</strong>
            <p>
              {data.projects.find((p) => p.id === k.project_id)?.name ||
                k.project_id}{" "}
              ·{" "}
              {k.inbox_id
                ? data.inboxes.find((b) => b.id === k.inbox_id)?.name ||
                  k.inbox_id
                : "Entire project"}
            </p>
            <p>{k.permissions.join(", ")}</p>
            <p>
              {k.revoked_at
                ? "Revoked"
                : Date.parse(k.expires_at) <= Date.now()
                  ? "Expired"
                  : `Expires ${new Date(k.expires_at).toLocaleString()}`}
            </p>
            {!k.revoked_at && (
              <button
                className="secondary"
                disabled={busy}
                onClick={() => {
                  void revoke(k.id);
                }}
                aria-label={`Revoke ${k.name}`}
              >
                Revoke
              </button>
            )}
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

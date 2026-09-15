import { useEffect, useState, type FormEvent } from "react";
import { api, type Membership, type User, type UserSession } from "../api";
import { Modal } from "./Modal";
import type { Workspaces } from "./WorkspacesDialog";

export function TeamDialog({
  data,
  operator,
  session,
  close,
}: {
  data: Workspaces;
  operator: boolean;
  session?: UserSession;
  close: () => void;
}) {
  const [project, setProject] = useState(data.projects[0]?.id || "");
  const [members, setMembers] = useState<Membership[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [username, setUsername] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [revision, setRevision] = useState(0);
  const canManage =
    operator ||
    session?.memberships.some(
      (m) => m.project_id === project && m.role === "admin",
    );
  useEffect(() => {
    const controller = new AbortController();
    if (project)
      void api<{ members: Membership[] }>(
        `/team/memberships?project_id=${encodeURIComponent(project)}`,
        { signal: controller.signal },
      )
        .then((v) => setMembers(v.members))
        .catch((e) => {
          if (!controller.signal.aborted) setError(e.message);
        });
    if (operator)
      void api<{ users: User[] }>("/team/users", { signal: controller.signal })
        .then((v) => setUsers(v.users))
        .catch((e) => {
          if (!controller.signal.aborted) setError(e.message);
        });
    return () => controller.abort();
  }, [project, operator, revision]);
  async function run(action: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
      setRevision((n) => n + 1);
      setNotice(message);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const input = Object.fromEntries(new FormData(form));
    await run(async () => {
      const user = await api<User>("/team/users", {
        method: "POST",
        body: JSON.stringify(input),
      });
      form.reset();
      setUsername(user.username);
    }, "User created. Assign a project role below.");
  }
  return (
    <Modal title="Team and project roles" close={close}>
      <p className="modal-description">
        Viewer: read and export. Member: capture and edit. Project admin: also
        delete, create inboxes and manage members. Changes to a member’s role
        end their existing sessions.
      </p>
      {operator && (
        <>
          <h3>Create an account</h3>
          <form
            onSubmit={(e) => {
              void create(e);
            }}
          >
            <label>
              Username
              <input
                name="username"
                required
                minLength={3}
                maxLength={64}
                pattern="[a-zA-Z0-9][a-zA-Z0-9._\-]{2,63}"
                autoComplete="off"
              />
            </label>
            <label>
              Display name
              <input name="name" required maxLength={80} />
            </label>
            <label>
              Initial password
              <input
                name="password"
                type="password"
                required
                minLength={12}
                maxLength={128}
                autoComplete="new-password"
              />
            </label>
            <button className="primary" disabled={busy}>
              Create user
            </button>
          </form>
        </>
      )}
      <h3>Project members</h3>
      <label>
        Project
        <select
          value={project}
          onChange={(e) => {
            setProject(e.target.value);
            setMembers([]);
            setError("");
          }}
        >
          {data.projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </label>
      {!project && (
        <p className="modal-description">No project membership assigned yet.</p>
      )}
      {canManage && project && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const form = new FormData(e.currentTarget);
            void run(
              () =>
                api("/team/memberships", {
                  method: "PUT",
                  body: JSON.stringify({
                    project_id: project,
                    username,
                    role: form.get("role"),
                  }),
                }),
              "Project role saved.",
            );
          }}
        >
          <label>
            Member username
            <input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              maxLength={64}
              autoComplete="off"
            />
          </label>
          <label>
            Role
            <select name="role" defaultValue="viewer">
              <option value="viewer">Viewer — read only</option>
              <option value="member">Member — capture and edit</option>
              <option value="admin">Project admin</option>
            </select>
          </label>
          <button className="secondary" disabled={busy}>
            Assign role
          </button>
        </form>
      )}
      <ul className="key-list">
        {members.map((m) => (
          <li key={m.user_id}>
            <strong>{m.name}</strong>
            <p>
              {m.username} · {m.role}
            </p>
            {canManage && (
              <button
                className="secondary"
                disabled={busy}
                aria-label={`Remove ${m.username} from project`}
                onClick={() => {
                  void run(
                    () =>
                      api(
                        `/team/memberships?project_id=${encodeURIComponent(project)}&user_id=${encodeURIComponent(m.user_id)}`,
                        { method: "DELETE" },
                      ),
                    "Membership removed.",
                  );
                }}
              >
                Remove member
              </button>
            )}
          </li>
        ))}
      </ul>
      {operator && (
        <>
          <h3>Accounts</h3>
          <ul className="key-list">
            {users.map((u) => (
              <li key={u.id}>
                <strong>{u.name}</strong>
                <p>
                  {u.username} · {u.disabled ? "Disabled" : "Active"}
                </p>
                <button
                  className="secondary"
                  disabled={busy}
                  aria-label={`${u.disabled ? "Enable" : "Disable"} ${u.username}`}
                  onClick={() => {
                    void run(
                      () =>
                        api(`/team/users/${u.id}`, {
                          method: "PATCH",
                          body: JSON.stringify({ disabled: !u.disabled }),
                        }),
                      "Account updated; existing sessions ended.",
                    );
                  }}
                >
                  {u.disabled ? "Enable" : "Disable"}
                </button>
              </li>
            ))}
          </ul>
          {users.length > 0 && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const form = e.currentTarget;
                const input = new FormData(form);
                void run(async () => {
                  await api(`/team/users/${input.get("id")}/password`, {
                    method: "PUT",
                    body: JSON.stringify({ password: input.get("password") }),
                  });
                  form.reset();
                }, "Password reset; existing sessions ended.");
              }}
            >
              <h3>Reset account password</h3>
              <label>
                Account
                <select name="id">
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.username}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                New account password
                <input
                  name="password"
                  type="password"
                  minLength={12}
                  maxLength={128}
                  required
                  autoComplete="new-password"
                />
              </label>
              <button className="secondary" disabled={busy}>
                Reset password
              </button>
            </form>
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
    </Modal>
  );
}

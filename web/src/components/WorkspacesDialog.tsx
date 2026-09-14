import { useState, type FormEvent } from "react";
import { api } from "../api";
import { Modal } from "./Modal";

export interface Workspaces {
  projects: { id: string; name: string }[];
  inboxes: { id: string; project_id: string; name: string }[];
}
export function WorkspacesDialog({
  data,
  select,
  close,
}: {
  data: Workspaces;
  select: (id: string) => void;
  close: () => void;
}) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function create(e: FormEvent<HTMLFormElement>, project: boolean) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const input = Object.fromEntries(new FormData(e.currentTarget));
      const result = await api<{ id?: string; inbox?: { id: string } }>(
        project ? "/projects" : "/inboxes",
        { method: "POST", body: JSON.stringify(input) },
      );
      select(project ? result.inbox!.id : result.id!);
      close();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="Projects and inboxes" close={close}>
      <p className="modal-description">
        Separate application traffic into inboxes. Existing integrations use the
        default local inbox.
      </p>
      <form
        onSubmit={(e) => {
          void create(e, true);
        }}
      >
        <label>
          New project name
          <input
            name="name"
            maxLength={80}
            required
            placeholder="e.g. Checkout"
          />
        </label>
        <button className="primary" disabled={busy}>
          Create project
        </button>
      </form>
      <h3>Add an inbox</h3>
      <form
        onSubmit={(e) => {
          void create(e, false);
        }}
      >
        <div className="form-row">
          <label>
            Project
            <select name="project_id">
              {data.projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Inbox name
            <input
              name="name"
              maxLength={80}
              required
              placeholder="e.g. End-to-end tests"
            />
          </label>
        </div>
        <button className="secondary" disabled={busy}>
          Create inbox
        </button>
      </form>
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
    </Modal>
  );
}

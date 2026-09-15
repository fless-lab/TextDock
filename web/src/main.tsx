import { StrictMode, useEffect, useRef, useState, type FormEvent } from "react";
import { createRoot } from "react-dom/client";
import { KeysDialog } from "./components/KeysDialog";
import { TeamDialog } from "./components/TeamDialog";
import { AccountDialog } from "./components/AccountDialog";
import {
  ArrowLeft,
  ArrowUpRight,
  Check,
  Code2,
  Copy,
  Download,
  Inbox,
  KeyRound,
  MessageSquare,
  Moon,
  Plus,
  Radio,
  Search,
  Settings2,
  Smartphone,
  Sun,
  Terminal,
  Trash2,
  Star,
  FolderOpen,
  ChevronLeft,
  ChevronRight,
  FlaskConical,
} from "lucide-react";
import { api, APIError, type Info, type Message } from "./api";
import "./styles.css";
import { Modal } from "./components/Modal";
import { ConnectDialog } from "./components/ConnectDialog";
import { PhoneApp } from "./PhoneApp";
import { useLiveEvents } from "./hooks/useLiveEvents";
import { copyText } from "./clipboard";
import { ScenariosDialog, type Scenario } from "./components/ScenariosDialog";
import { MessageEvents } from "./components/MessageEvents";
import { DeviceLabDialog } from "./components/DeviceLabDialog";
import {
  RelayDialog,
  RelayDetails,
  type RelayInfo,
} from "./components/RelayDialog";
import {
  WorkspacesDialog,
  type Workspaces,
} from "./components/WorkspacesDialog";

function App() {
  const [info, setInfo] = useState<Info>();
  const [locked, setLocked] = useState(false);
  const [loginMode, setLoginMode] = useState<"operator" | "user">("operator");
  const authRevision = useRef(0);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [selected, setSelected] = useState<string>();
  const [query, setQuery] = useState("");
  const [otpOnly, setOtpOnly] = useState(false);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [composeMode, setComposeMode] = useState("capture");
  const [relay, setRelay] = useState<RelayInfo>({
    enabled: false,
    driver: "",
    limit_per_minute: 10,
  });
  const intentKey = useRef("");
  const [inbox, setInbox] = useState("local");
  const [workspaces, setWorkspaces] = useState<Workspaces>({
    projects: [{ id: "default", name: "Local project" }],
    inboxes: [{ id: "local", project_id: "default", name: "Inbox" }],
  });
  const [favorites, setFavorites] = useState(false);
  const [filters, setFilters] = useState({
    to: "",
    run_id: "",
    tag: "",
    since: "",
  });
  const [cursors, setCursors] = useState<string[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [checked, setChecked] = useState<string[]>([]);
  const cursor = cursors.at(-1) || "";
  const [refresh, setRefresh] = useState(0);
  const [modal, setModal] = useState<
    | "compose"
    | "connect"
    | "settings"
    | "integrate"
    | "workspaces"
    | "commands"
    | "scenarios"
    | "relay"
    | "device-lab"
    | "keys"
    | "team"
    | "account"
  >();
  const [tab, setTab] = useState<"message" | "json" | "events">("message");
  const [busy, setBusy] = useState(false);
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem("textdock-theme") ||
      (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  );
  const search = useRef<HTMLInputElement>(null);
  const operator = info?.operator !== false;
  const activeProject =
    workspaces.inboxes.find((b) => b.id === inbox)?.project_id || "";
  const role = info?.session?.memberships.find(
    (m) => m.project_id === activeProject,
  )?.role;
  const canRead =
    operator ||
    !!role ||
    !!info?.api_key?.permissions.includes("messages:read");
  const canWrite =
    operator ||
    role === "member" ||
    role === "admin" ||
    !!info?.api_key?.permissions.includes("messages:write");
  const canDelete =
    operator ||
    role === "admin" ||
    !!info?.api_key?.permissions.includes("messages:delete");
  useEffect(() => {
    const controller = new AbortController();
    if (info && !locked && operator)
      void api<RelayInfo>("/relay")
        .then((v) => {
          if (!controller.signal.aborted) setRelay(v);
        })
        .catch((e) => {
          if (!controller.signal.aborted) setError(e.message);
        });
    return () => controller.abort();
  }, [info, locked, operator]);
  useEffect(() => {
    if (modal === "compose")
      intentKey.current = Array.from(crypto.getRandomValues(new Uint32Array(4)))
        .map((n) => n.toString(16))
        .join("-");
  }, [modal]);
  useEffect(() => {
    const controller = new AbortController();
    if (info && !locked && operator)
      void api<{ scenarios: Scenario[] }>(
        `/scenarios?inbox=${encodeURIComponent(inbox)}`,
      )
        .then((data) => {
          if (!controller.signal.aborted) setScenarios(data.scenarios);
        })
        .catch((e) => {
          if (!controller.signal.aborted) setError(e.message);
        });
    return () => controller.abort();
  }, [info, locked, inbox, refresh, operator]);
  const parameters = new URLSearchParams({
    inbox,
    q: query,
    cursor,
    otp: String(otpOnly),
    favorite: String(favorites),
    to: filters.to,
    run_id: filters.run_id,
    tag: filters.tag,
  });
  if (filters.since)
    parameters.set("since", new Date(filters.since).toISOString());
  const queryString = parameters.toString();
  function resetPage() {
    setCursors([]);
    setSelected(undefined);
    setChecked([]);
  }
  function chooseInbox(id: string) {
    authRevision.current++;
    setBusy(false);
    setTab("message");
    setInbox(id);
    setMessages([]);
    setFilters({ to: "", run_id: "", tag: "", since: "" });
    setQuery("");
    setFavorites(false);
    setOtpOnly(false);
    resetPage();
    setRefresh((n) => n + 1);
  }
  useEffect(() => {
    const controller = new AbortController();
    if (info && !locked)
      void api<Workspaces>("/workspaces", { signal: controller.signal })
        .then((data) => {
          if (!controller.signal.aborted) {
            setWorkspaces(data);
            if (!data.inboxes.some((b) => b.id === inbox))
              chooseInbox(data.inboxes[0]?.id || "");
          }
        })
        .catch((e) => {
          if (!controller.signal.aborted) setError(e.message);
        });
    return () => controller.abort();
  }, [info, locked, refresh]);
  const live = useLiveEvents(
    info && !locked
      ? canRead && inbox
        ? `/api/v1/events?inbox=${encodeURIComponent(inbox)}`
        : info.session
          ? "/api/v1/account/events"
          : undefined
      : undefined,
    sessionStorage.getItem("textdock-token"),
  );
  useEffect(() => {
    if (live.state === "unauthorized") {
      endSession();
    }
  }, [live.state]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("textdock-theme", theme);
  }, [theme]);
  useEffect(() => {
    const shortcut = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k" && !locked) {
        e.preventDefault();
        setModal((current) =>
          current === "commands" ? undefined : "commands",
        );
        return;
      }
      const target = e.target as HTMLElement;
      if (
        e.key === "/" &&
        !["INPUT", "TEXTAREA"].includes(target.tagName) &&
        !target.isContentEditable &&
        !modal &&
        !locked
      ) {
        e.preventDefault();
        search.current?.focus();
      }
    };
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, [modal, locked]);
  useEffect(() => {
    if (notice) {
      const timer = setTimeout(() => setNotice(""), 3500);
      return () => clearTimeout(timer);
    }
  }, [notice]);

  async function loadInfo() {
    const revision = ++authRevision.current;
    try {
      const next = await api<Info>("/info");
      const data = await api<Workspaces>("/workspaces");
      if (revision !== authRevision.current) return;
      setWorkspaces(data);
      chooseInbox(
        data.inboxes.some((b) => b.id === inbox)
          ? inbox
          : data.inboxes[0]?.id || "",
      );
      setInfo(next);
      setLocked(false);
      setError("");
    } catch (e) {
      if (revision !== authRevision.current) return;
      if (e instanceof APIError && e.status === 401) endSession();
      setError((e as Error).message);
    }
  }
  function endSession() {
    authRevision.current++;
    setBusy(false);
    setTab("message");
    sessionStorage.removeItem("textdock-token");
    setLocked(true);
    setInfo(undefined);
    setMessages([]);
    setSelected(undefined);
    setChecked([]);
    setScenarios([]);
    setRelay({ enabled: false, driver: "", limit_per_minute: 10 });
    setComposeMode("capture");
    setWorkspaces({ projects: [], inboxes: [] });
    setModal(undefined);
    setNotice("");
  }
  async function signIn(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const input = new FormData(event.currentTarget);
    try {
      let token = String(input.get("token") || "");
      if (loginMode === "user")
        token = (
          await api<{ token: string }>("/auth/login", {
            method: "POST",
            body: JSON.stringify({
              username: input.get("username"),
              password: input.get("password"),
            }),
          })
        ).token;
      sessionStorage.setItem("textdock-token", token);
      await loadInfo();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    void loadInfo();
  }, []);
  useEffect(() => {
    if (!info || locked || !inbox || !canRead) return;
    let stopped = false;
    const controller = new AbortController();
    async function poll() {
      const revision = authRevision.current;
      try {
        const data = await api<{ messages: Message[]; next_cursor: string }>(
          `/messages?${queryString}`,
          { signal: controller.signal },
        );
        if (!stopped && revision === authRevision.current) {
          setMessages(data.messages);
          setNextCursor(data.next_cursor);
          setError("");
        }
      } catch (e) {
        if (!stopped && revision === authRevision.current) {
          if (e instanceof APIError && e.status === 401) {
            endSession();
          }
          setError((e as Error).message);
        }
      }
    }
    const debounce = setTimeout(poll, 150);
    return () => {
      stopped = true;
      clearTimeout(debounce);
      controller.abort();
    };
  }, [info, locked, queryString, refresh, live.revision, canRead]);

  const visible = messages;
  const current = visible.find((m) => m.id === selected);

  async function copy(text: string) {
    setNotice(
      (await copyText(text))
        ? "Copied to clipboard"
        : "Select the text to copy it.",
    );
  }
  async function send(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!canWrite || !inbox) return;
    const revision = authRevision.current;
    setBusy(true);
    const data = new FormData(e.currentTarget);
    try {
      const m = await api<Message>(
        composeMode === "inbound" ? "/inbound" : "/messages",
        {
          method: "POST",
          body: JSON.stringify({
            ...Object.fromEntries(data),
            inbox,
            mode: composeMode === "inbound" ? "simulate" : composeMode,
            ...(composeMode === "relay"
              ? { idempotency_key: intentKey.current }
              : {}),
          }),
        },
      );
      if (revision !== authRevision.current) return;
      setQuery("");
      setOtpOnly(false);
      setFavorites(false);
      setFilters({ to: "", run_id: "", tag: "", since: "" });
      resetPage();
      setMessages((previous) => [
        m,
        ...previous.filter((item) => item.id !== m.id),
      ]);
      setSelected(m.id);
      setTab("message");
      setModal(undefined);
      setRefresh((n) => n + 1);
      setNotice(
        composeMode === "capture"
          ? "Message captured."
          : composeMode === "relay"
            ? "Real SMS queued."
            : "Simulation started.",
      );
    } catch (e) {
      if (revision === authRevision.current) setError((e as Error).message);
    } finally {
      if (revision === authRevision.current) setBusy(false);
    }
  }
  async function remove(m: Message) {
    if (!canDelete) return;
    const revision = authRevision.current;
    setBusy(true);
    try {
      await api(`/messages/${m.id}`, { method: "DELETE" });
      if (revision !== authRevision.current) return;
      setSelected(undefined);
      setMessages((previous) => previous.filter((item) => item.id !== m.id));
      setRefresh((n) => n + 1);
      setNotice("Message deleted");
    } catch (e) {
      if (revision === authRevision.current) setError((e as Error).message);
    } finally {
      if (revision === authRevision.current) setBusy(false);
    }
  }
  function download(m: Message) {
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(m, null, 2)], { type: "application/json" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = `${m.id}.json`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  async function update(
    m: Message,
    patch: { favorite?: boolean; tags?: string[] },
  ) {
    if (!canWrite) return;
    const revision = authRevision.current;
    try {
      const updated = await api<Message>(`/messages/${m.id}`, {
        method: "PATCH",
        body: JSON.stringify(patch),
      });
      if (revision !== authRevision.current) return;
      setMessages((items) =>
        items.map((item) => (item.id === updated.id ? updated : item)),
      );
      setRefresh((n) => n + 1);
    } catch (e) {
      if (revision === authRevision.current) setError((e as Error).message);
    }
  }
  async function deleteChecked() {
    if (!canDelete) return;
    const revision = authRevision.current;
    setBusy(true);
    try {
      await api("/messages/delete", {
        method: "POST",
        body: JSON.stringify({ ids: checked }),
      });
      if (revision !== authRevision.current) return;
      resetPage();
      setRefresh((n) => n + 1);
      setNotice("Selected messages deleted.");
    } catch (e) {
      if (revision === authRevision.current) setError((e as Error).message);
    } finally {
      if (revision === authRevision.current) setBusy(false);
    }
  }
  async function exportPage() {
    const revision = authRevision.current;
    try {
      const token = sessionStorage.getItem("textdock-token");
      const response = await fetch(`/api/v1/export?${queryString}&format=csv`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error("Export failed");
      const blob = await response.blob();
      if (revision !== authRevision.current) return;
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "textdock.csv";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      if (revision === authRevision.current) setError((e as Error).message);
    }
  }

  const curl = [
    `curl ${location.origin}/api/v1/messages`,
    "  -H 'Content-Type: application/json'",
    ...(info?.auth_enabled
      ? ["  -H 'Authorization: Bearer YOUR_TEXTDOCK_TOKEN'"]
      : []),
    `  -d '{"inbox":"${inbox}","to":"+33612345678","from":"Acme","body":"Your code is 482193","run_id":"signup-1"}'`,
  ].join(" \\\n");

  if (locked)
    return (
      <main className="gate">
        <div className="brand-mark">
          <MessageSquare />
        </div>
        <h1>TextDock</h1>
        <p>
          {loginMode === "operator"
            ? "Enter this server’s TextDock token to open the inbox."
            : "Sign in with the account created by your server operator."}
        </p>
        <form
          onSubmit={(e) => {
            void signIn(e);
          }}
        >
          {loginMode === "operator" ? (
            <label>
              Server token
              <input
                name="token"
                type="password"
                autoComplete="current-password"
                required
                autoFocus
              />
            </label>
          ) : (
            <>
              <label>
                Username
                <input
                  name="username"
                  required
                  maxLength={64}
                  autoComplete="username"
                  autoFocus
                />
              </label>
              <label>
                Password
                <input
                  name="password"
                  type="password"
                  required
                  minLength={12}
                  maxLength={128}
                  autoComplete="current-password"
                />
              </label>
            </>
          )}
          <button className="primary" disabled={busy}>
            {loginMode === "operator" ? "Unlock inbox" : "Sign in"}{" "}
            <ArrowUpRight size={16} />
          </button>
        </form>
        <button
          className="secondary"
          disabled={busy}
          onClick={() => {
            setLoginMode(loginMode === "operator" ? "user" : "operator");
            setError("");
          }}
        >
          {loginMode === "operator"
            ? "Sign in with a user account"
            : "Use operator token"}
        </button>
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
      </main>
    );

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="/">
          <span className="brand-mark">
            <MessageSquare size={22} />
          </span>
          TextDock
        </a>
        <nav aria-label="Workspace">
          <button
            className={!otpOnly && !favorites ? "nav-item active" : "nav-item"}
            onClick={() => {
              setOtpOnly(false);
              setFavorites(false);
              resetPage();
              setSelected(undefined);
            }}
          >
            <Inbox size={18} />
            All messages<span className="nav-count">{messages.length}</span>
          </button>
          <button
            className={otpOnly ? "nav-item active" : "nav-item"}
            onClick={() => {
              setOtpOnly(true);
              setFavorites(false);
              resetPage();
              setSelected(undefined);
            }}
          >
            <KeyRound size={18} />
            OTP codes
          </button>
          <button
            className={favorites ? "nav-item active" : "nav-item"}
            onClick={() => {
              setFavorites(true);
              setOtpOnly(false);
              resetPage();
            }}
          >
            <Star size={18} />
            Favorites
          </button>
          <button className="nav-item" onClick={() => setModal("integrate")}>
            <Code2 size={18} />
            Integration
          </button>
          {operator && (
            <>
              <button
                className="nav-item"
                onClick={() => setModal("scenarios")}
              >
                <FlaskConical size={18} />
                Scenarios
              </button>
              <button className="nav-item" onClick={() => setModal("relay")}>
                <Smartphone size={18} />
                Relay
              </button>
              <button
                className="nav-item"
                onClick={() => setModal("device-lab")}
              >
                <Terminal size={18} />
                Device lab
              </button>
              <button className="nav-item" onClick={() => setModal("connect")}>
                <Smartphone size={18} />
                Open on phone
              </button>
            </>
          )}
        </nav>
        <div className="sidebar-bottom">
          <div className="sidebar-footer">
            <button className="nav-item" onClick={() => setModal("settings")}>
              <Settings2 size={17} />
              Settings
            </button>
            <button
              className="icon-button"
              aria-label="Toggle color theme"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            >
              {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </button>
          </div>
          <small className="version">{info?.version || "…"}</small>
        </div>
      </aside>

      <main className="main">
        <header className="topbar">
          <div className="workspace-picker">
            <select
              aria-label="Inbox"
              value={inbox}
              onChange={(e) => chooseInbox(e.target.value)}
            >
              {workspaces.projects.map((project) => (
                <optgroup key={project.id} label={project.name}>
                  {workspaces.inboxes
                    .filter((item) => item.project_id === project.id)
                    .map((item) => (
                      <option value={item.id} key={item.id}>
                        {project.name} / {item.name}
                      </option>
                    ))}
                </optgroup>
              ))}
            </select>
            <button
              className="icon-button"
              aria-label="Manage projects"
              onClick={() => setModal("workspaces")}
            >
              <FolderOpen size={17} />
            </button>
          </div>
          <div className="connection">
            {info?.session && (
              <button
                className="text-button"
                onClick={() => setModal("account")}
              >
                Your account
              </button>
            )}
            <span
              className={
                error || live.state !== "connected"
                  ? "live-dot offline"
                  : "live-dot"
              }
            />
            {!inbox
              ? "No inbox access"
              : error
                ? "Connection interrupted"
                : info
                  ? live.state === "connected"
                    ? "Connected"
                    : "Reconnecting…"
                  : "Connecting…"}
          </div>
        </header>
        <section className="page-heading">
          <div>
            <h1>
              {otpOnly
                ? "OTP codes"
                : favorites
                  ? "Favorites"
                  : "Message inbox"}
              <span>{visible.length}</span>
            </h1>
            <p>
              {relay.enabled
                ? "Capture, simulation and real SMS relay"
                : "Local messages · No real SMS delivery"}
            </p>
          </div>
          <button
            className="primary"
            disabled={!canWrite || !inbox}
            onClick={() => {
              setError("");
              setModal("compose");
            }}
          >
            <Plus size={17} />
            Send test SMS
          </button>
        </section>
        {info?.session && !inbox && (
          <p role="status" className="modal-description">
            No project access assigned. Ask your operator or project
            administrator to add your account.
          </p>
        )}
        {error && (
          <div role="alert" className="error-banner">
            {error}
            <button
              onClick={() => {
                void loadInfo();
                setRefresh((n) => n + 1);
              }}
            >
              Retry
            </button>
          </div>
        )}
        <div className="inbox-toolbar">
          <label className="search">
            <Search size={17} />
            <input
              ref={search}
              aria-label="Search messages"
              placeholder="Search messages or phone numbers…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                resetPage();
              }}
            />
            <kbd>/</kbd>
          </label>
          <span className="toolbar-note">
            <Radio size={14} />{" "}
            {live.state === "connected" ? "Live updates" : "Reconnecting…"}
          </span>
        </div>

        <div className="filter-toolbar">
          <details className="filter-menu">
            <summary>
              Filters{Object.values(filters).some(Boolean) ? " · active" : ""}
            </summary>
            <div className="filter-fields">
              {(["to", "run_id", "tag", "since"] as const).map((key) => (
                <label key={key}>
                  {
                    {
                      to: "Recipient",
                      run_id: "Test run",
                      tag: "Tag",
                      since: "Received since",
                    }[key]
                  }
                  <input
                    type={key === "since" ? "datetime-local" : "text"}
                    value={filters[key]}
                    onChange={(e) => {
                      setFilters((previous) => ({
                        ...previous,
                        [key]: e.target.value,
                      }));
                      resetPage();
                    }}
                  />
                </label>
              ))}
              <button
                className="text-button"
                onClick={() => {
                  setFilters({ to: "", run_id: "", tag: "", since: "" });
                  resetPage();
                }}
              >
                Clear filters
              </button>
            </div>
          </details>
          <button
            className="text-button"
            disabled={!canRead || !inbox}
            onClick={() => {
              void exportPage();
            }}
          >
            <Download size={14} />
            Export page
          </button>
          {checked.length > 0 && (
            <button
              className="text-button danger"
              disabled={busy || !canDelete}
              onClick={() => {
                void deleteChecked();
              }}
            >
              <Trash2 size={14} />
              Delete selected ({checked.length})
            </button>
          )}
        </div>

        <div className={`inbox-layout ${current ? "has-selection" : ""}`}>
          <section className="message-list" aria-label="Messages">
            <div className="list-heading">
              <label className="select-all">
                <input
                  type="checkbox"
                  aria-label="Select page"
                  disabled={!canDelete}
                  checked={
                    visible.length > 0 &&
                    visible.every((m) => checked.includes(m.id))
                  }
                  onChange={(e) =>
                    setChecked(e.target.checked ? visible.map((m) => m.id) : [])
                  }
                />
                {otpOnly ? "With OTP" : "Latest messages"}
              </label>
              <span>{visible.length} on this page</span>
            </div>
            {visible.length ? (
              visible.map((m) => (
                <div key={m.id} className="message-item">
                  <input
                    className="message-checkbox"
                    type="checkbox"
                    aria-label={`Select message ${m.id}`}
                    disabled={!canDelete}
                    checked={checked.includes(m.id)}
                    onChange={(e) =>
                      setChecked((previous) =>
                        e.target.checked
                          ? [...previous, m.id]
                          : previous.filter((id) => id !== m.id),
                      )
                    }
                  />
                  <button
                    className={`message-row ${current?.id === m.id ? "selected" : ""}`}
                    onClick={() => {
                      setSelected(m.id);
                      setTab("message");
                    }}
                  >
                    <span className="message-summary">
                      <span className="row-title">
                        <strong>
                          {m.to}
                          {m.favorite && (
                            <Star className="favorite-mark" size={12} />
                          )}
                        </strong>
                        <time>
                          {new Date(m.created_at).toLocaleTimeString([], {
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </time>
                      </span>
                      <span className="row-body">{m.body}</span>
                      <span className="row-tags">
                        {m.mode === "relay" && (
                          <span className="simulation-tag">
                            real · {m.status}
                          </span>
                        )}
                        {m.mode === "simulate" && (
                          <span className="simulation-tag">
                            {m.direction === "inbound" ? "inbound" : m.status}
                          </span>
                        )}
                        <span className="tag">{m.from}</span>
                        {m.analysis.otp && (
                          <span className="otp-tag">
                            <KeyRound size={11} />
                            OTP
                          </span>
                        )}
                        <span className="source-label">{m.source}</span>
                      </span>
                    </span>
                  </button>
                </div>
              ))
            ) : (
              <div className="list-empty">
                <Inbox size={26} />
                <h3>
                  {query || otpOnly
                    ? "No matching messages"
                    : "No messages yet"}
                </h3>
                <p>
                  {query || otpOnly
                    ? "Try another search or send a test message."
                    : "Send a message through the API or create a test SMS."}
                </p>
                <button
                  className="text-button"
                  onClick={() => setModal("compose")}
                  disabled={!canWrite || !inbox}
                >
                  Send test SMS <Plus size={14} />
                </button>
              </div>
            )}
            <div className="pagination">
              <button
                className="icon-button"
                aria-label="Previous page"
                disabled={!cursors.length}
                onClick={() => {
                  setCursors((stack) => stack.slice(0, -1));
                  setSelected(undefined);
                  setChecked([]);
                }}
              >
                <ChevronLeft size={17} />
              </button>
              <span>Page {cursors.length + 1}</span>
              <button
                className="icon-button"
                aria-label="Next page"
                disabled={!nextCursor}
                onClick={() => {
                  setCursors((stack) => [...stack, nextCursor]);
                  setSelected(undefined);
                  setChecked([]);
                }}
              >
                <ChevronRight size={17} />
              </button>
            </div>
          </section>

          <section className="detail" aria-label="Message details">
            {current ? (
              <>
                <header className="detail-header">
                  <button
                    className="icon-button mobile-back"
                    aria-label="Back to messages"
                    onClick={() => setSelected(undefined)}
                  >
                    <ArrowLeft size={20} />
                  </button>
                  <div>
                    <h2>{current.to}</h2>
                    <span>From {current.from}</span>
                  </div>
                  <div className="detail-actions">
                    <button
                      className="icon-button"
                      aria-label={
                        current.favorite
                          ? "Remove from favorites"
                          : "Add to favorites"
                      }
                      disabled={!canWrite}
                      onClick={() => {
                        void update(current, { favorite: !current.favorite });
                      }}
                    >
                      <Star
                        size={17}
                        fill={current.favorite ? "currentColor" : "none"}
                      />
                    </button>
                    <button
                      className="icon-button"
                      onClick={() => download(current)}
                      aria-label="Export message"
                    >
                      <Download size={17} />
                    </button>
                    <button
                      className="icon-button danger"
                      disabled={busy || !canDelete}
                      onClick={() => {
                        void remove(current);
                      }}
                      aria-label="Delete message"
                    >
                      <Trash2 size={17} />
                    </button>
                  </div>
                </header>
                <div
                  className="detail-tabs"
                  role="tablist"
                  aria-label="Message view"
                >
                  <button
                    role="tab"
                    aria-selected={tab === "message"}
                    onClick={() => setTab("message")}
                  >
                    Message
                  </button>
                  <button
                    role="tab"
                    aria-selected={tab === "json"}
                    onClick={() => setTab("json")}
                  >
                    Raw JSON
                  </button>
                  <button
                    role="tab"
                    aria-selected={tab === "events"}
                    onClick={() => setTab("events")}
                  >
                    Events
                  </button>
                  <span className="status-pill">
                    <Check size={12} />
                    {current.status}
                  </span>
                </div>
                <div className="detail-content">
                  {tab === "events" ? (
                    <>
                      {operator && current.mode === "relay" && (
                        <RelayDetails
                          id={current.id}
                          revision={live.revision}
                        />
                      )}
                      <MessageEvents
                        id={current.id}
                        revision={live.revision}
                        operator={operator}
                      />
                    </>
                  ) : tab === "json" ? (
                    <div className="raw-view">
                      <button
                        className="secondary"
                        onClick={() => {
                          void copy(JSON.stringify(current, null, 2));
                        }}
                      >
                        <Copy size={14} />
                        Copy JSON
                      </button>
                      <pre>{JSON.stringify(current, null, 2)}</pre>
                    </div>
                  ) : (
                    <>
                      <div className="conversation-date">
                        {new Date(current.created_at).toLocaleString([], {
                          dateStyle: "medium",
                          timeStyle: "short",
                        })}
                      </div>
                      <div className="sms-bubble">{current.body}</div>
                      <div className="bubble-caption">
                        {current.mode === "relay"
                          ? `Real SMS relay · ${current.status}`
                          : current.mode === "simulate"
                            ? `Simulated ${current.direction}`
                            : "Captured locally"}
                        {current.mode !== "relay" && " · No SMS sent"}
                      </div>
                      {current.analysis.otp && (
                        <div className="otp-card">
                          <div>
                            <span>Detected code</span>
                            <strong>{current.analysis.otp}</strong>
                            <small>Suggested by text analysis</small>
                          </div>
                          <button
                            className="secondary"
                            onClick={() => {
                              void copy(current.analysis.otp!);
                            }}
                          >
                            <Copy size={14} />
                            Copy code
                          </button>
                        </div>
                      )}
                      <div className="section-label">Details</div>
                      <div className="insight-grid">
                        <div>
                          <span>Encoding</span>
                          <strong>{current.analysis.encoding}</strong>
                        </div>
                        <div>
                          <span>Characters</span>
                          <strong>{current.analysis.characters}</strong>
                        </div>
                        <div>
                          <span>SMS segments</span>
                          <strong>
                            {current.analysis.segments}
                            <small> estimated</small>
                          </strong>
                        </div>
                      </div>
                      {current.analysis.non_gsm?.length ? (
                        <p className="encoding-note">
                          Unicode encoding triggered by:{" "}
                          <code>{current.analysis.non_gsm.join(" ")}</code>
                        </p>
                      ) : null}
                      <dl className="metadata">
                        <div>
                          <dt>Inbox</dt>
                          <dd>{current.inbox}</dd>
                        </div>
                        <div>
                          <dt>Message ID</dt>
                          <dd>{current.id}</dd>
                        </div>
                        <div>
                          <dt>Source</dt>
                          <dd>{current.source}</dd>
                        </div>
                        <div>
                          <dt>Test run</dt>
                          <dd>{current.run_id || "Not specified"}</dd>
                        </div>
                        <div>
                          <dt>Delivery</dt>
                          <dd>
                            {current.mode === "relay"
                              ? current.status
                              : "Not sent"}
                          </dd>
                        </div>
                      </dl>
                      <form
                        key={current.id + current.tags.join(",")}
                        className="tags-editor"
                        onSubmit={(e) => {
                          e.preventDefault();
                          const raw = String(
                            new FormData(e.currentTarget).get("tags") || "",
                          );
                          void update(current, {
                            tags: raw
                              .split(",")
                              .map((tag) => tag.trim())
                              .filter(Boolean),
                          });
                        }}
                      >
                        <label>
                          Tags
                          <input
                            name="tags"
                            defaultValue={current.tags.join(", ")}
                            placeholder="Comma-separated tags"
                          />
                        </label>
                        <button className="secondary" disabled={!canWrite}>
                          Save tags
                        </button>
                      </form>
                      <button
                        className="text-button"
                        onClick={() => {
                          setFilters((previous) => ({
                            ...previous,
                            to: current.to,
                          }));
                          resetPage();
                        }}
                      >
                        View conversation with this recipient
                      </button>
                    </>
                  )}
                </div>
              </>
            ) : (
              <div className="welcome">
                <h2>Select a message</h2>
                <p>
                  Its content, detected code and technical details appear here.
                </p>
                <button
                  className="secondary"
                  onClick={() => setModal("integrate")}
                >
                  <Terminal size={16} />
                  API integration
                </button>
              </div>
            )}
          </section>
        </div>
      </main>

      {notice && (
        <div className="toast" role="status">
          <Check size={17} />
          {notice}
        </div>
      )}
      {modal === "compose" && (
        <Modal title="Send a test SMS" close={() => setModal(undefined)}>
          <p className="modal-description">
            Create a local message. Nothing will be sent to a mobile network.
          </p>
          <form
            onSubmit={(e) => {
              void send(e);
            }}
          >
            <label>
              Mode
              <select
                value={composeMode}
                onChange={(e) => setComposeMode(e.target.value)}
              >
                <option value="capture">Capture only</option>
                <option value="simulate" disabled={!operator}>
                  Simulate delivery
                </option>
                <option value="inbound" disabled={!operator}>
                  Simulate incoming SMS
                </option>
                <option value="relay" disabled={!operator || !relay.enabled}>
                  Relay — real SMS{!relay.enabled ? " (disabled)" : ""}
                </option>
              </select>
            </label>
            {composeMode === "simulate" && (
              <label>
                Scenario
                <select name="scenario_id" required defaultValue="">
                  <option value="" disabled>
                    Choose a scenario
                  </option>
                  {scenarios.map((scenario) => (
                    <option key={scenario.id} value={scenario.id}>
                      {scenario.name}
                    </option>
                  ))}
                </select>
                {!scenarios.length && (
                  <span className="optional">
                    Create a scenario from the sidebar first.
                  </span>
                )}
              </label>
            )}
            {composeMode === "relay" && (
              <p className="modal-description">
                Real sending through {relay.driver}.{" "}
                {relay.driver === "android"
                  ? "The default gateway SIM controls the sender."
                  : "Use an authorized sender number or ID."}{" "}
                This request has a reusable idempotency key until submission
                succeeds.
              </p>
            )}
            {composeMode !== "capture" && composeMode !== "relay" && (
              <label>
                Callback URL <span className="optional">optional</span>
                <input
                  name="callback_url"
                  type="url"
                  placeholder={
                    composeMode === "inbound"
                      ? "Your application’s inbound webhook URL"
                      : "Override scenario callback URL"
                  }
                />
              </label>
            )}
            <div className="form-row">
              <label>
                To
                <input
                  name="to"
                  type="tel"
                  defaultValue="+33612345678"
                  pattern="\+[1-9][0-9]{6,14}"
                  required
                />
              </label>
              <label>
                From
                <input
                  name="from"
                  defaultValue="Acme"
                  maxLength={64}
                  required
                />
              </label>
            </div>
            <label>
              Message
              <textarea
                name="body"
                defaultValue="Your TextDock verification code is 482193. It expires in 5 minutes."
                maxLength={4096}
                required
                rows={4}
              />
            </label>
            <label>
              Test run ID <span className="optional">optional</span>
              <input
                name="run_id"
                placeholder="e.g. signup-e2e-42"
                maxLength={128}
              />
            </label>
            {error && (
              <p role="alert" className="error-text">
                {error}
              </p>
            )}
            <div className="modal-footer">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(undefined)}
              >
                Cancel
              </button>
              <button className="primary" disabled={busy}>
                <Plus size={16} />
                {busy
                  ? "Submitting…"
                  : composeMode === "relay"
                    ? "Send real SMS"
                    : "Capture message"}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {modal === "integrate" && (
        <Modal
          title="Connect your application"
          close={() => setModal(undefined)}
        >
          <p className="modal-description">
            Send JSON to the capture API. Add a unique run_id to retrieve an OTP
            in your automated tests.
          </p>
          <pre className="code-block">{curl}</pre>
          <button
            className="secondary"
            onClick={() => {
              void copy(curl);
            }}
          >
            <Copy size={15} />
            Copy example
          </button>
          <h3>Node.js backend SDK</h3>
          <pre className="code-block">{`import { fromEnv } from '@textdock/sdk';\nconst sms = fromEnv();\nawait sms.send({\n  to: '+33612345678', from: 'Acme',\n  body: 'Your code is 482193', runId: 'signup-1'\n});`}</pre>
          <p className="modal-description">
            Set SMS_DRIVER to local, twilio, vonage or ovh. The SDK is available
            as a package in the GitHub release; production drivers require
            provider credentials.
          </p>
          <h3>Wait for a verification code</h3>
          <pre className="code-block">
            GET /api/v1/otp?to=%2B33612345678&amp;run_id=signup-1&amp;timeout=30
          </pre>
          <p className="modal-description">
            Returns a detected code and message ID. This tests your application
            flow; native SMS autofill requires a real SMS or platform emulator
            tooling.
          </p>
        </Modal>
      )}
      {modal === "connect" && (
        <ConnectDialog inbox={inbox} close={() => setModal(undefined)} />
      )}
      {modal === "scenarios" && (
        <ScenariosDialog
          inbox={inbox}
          scenarios={scenarios}
          changed={() => setRefresh((n) => n + 1)}
          close={() => setModal(undefined)}
        />
      )}
      {modal === "relay" && (
        <RelayDialog
          info={relay}
          version={info?.version}
          close={() => setModal(undefined)}
        />
      )}
      {modal === "device-lab" && (
        <DeviceLabDialog
          inbox={inbox}
          close={() => setModal(undefined)}
          changed={() => setRefresh((n) => n + 1)}
        />
      )}
      {modal === "workspaces" && (
        <WorkspacesDialog
          data={workspaces}
          select={chooseInbox}
          openKeys={() => setModal("keys")}
          openTeam={() => setModal("team")}
          operator={operator}
          adminProjects={
            info?.session?.memberships
              .filter((m) => m.role === "admin")
              .map((m) => m.project_id) || []
          }
          close={() => setModal(undefined)}
        />
      )}
      {modal === "keys" && operator && (
        <KeysDialog
          data={workspaces}
          inbox={inbox}
          authEnabled={!!info?.auth_enabled}
          close={() => setModal(undefined)}
        />
      )}
      {modal === "commands" && (
        <Modal title="Commands" close={() => setModal(undefined)}>
          <div className="command-list">
            <button
              disabled={!canWrite || !inbox}
              onClick={() => setModal("compose")}
            >
              Send test SMS
            </button>
            {operator && (
              <button onClick={() => setModal("connect")}>
                Connect a phone
              </button>
            )}
            <button onClick={() => setModal("workspaces")}>
              Manage projects and inboxes
            </button>
            <button onClick={() => setModal("integrate")}>
              API integration
            </button>
            {operator && (
              <button onClick={() => setModal("keys")}>Manage API keys</button>
            )}
            <button onClick={() => setModal("team")}>Manage team</button>
            {info?.session && (
              <button onClick={() => setModal("account")}>Your account</button>
            )}
            <button
              onClick={() => {
                setTheme(theme === "dark" ? "light" : "dark");
                setModal(undefined);
              }}
            >
              Toggle theme
            </button>
          </div>
        </Modal>
      )}
      {modal === "settings" && (
        <Modal title="Local workspace" close={() => setModal(undefined)}>
          <dl className="metadata">
            <div>
              <dt>Version</dt>
              <dd>{info?.version}</dd>
            </div>
            <div>
              <dt>Endpoint</dt>
              <dd>{location.origin}</dd>
            </div>
            <div>
              <dt>Mode</dt>
              <dd>Local capture and simulation</dd>
            </div>
            <div>
              <dt>Authentication</dt>
              <dd>
                {info?.auth_enabled
                  ? "Server token enabled"
                  : "Local access without token"}
              </dd>
            </div>
            <div>
              <dt>Storage</dt>
              <dd>SQLite · configured by server</dd>
            </div>
          </dl>
          <p className="modal-description">
            Configure the server using TEXTDOCK_LISTEN, TEXTDOCK_DB and
            TEXTDOCK_TOKEN. No cloud account is needed.
          </p>
          {operator && (
            <button className="secondary" onClick={() => setModal("keys")}>
              Manage API keys
            </button>
          )}
          <button className="secondary" onClick={() => setModal("team")}>
            Manage team
          </button>
          {info?.session && (
            <button className="secondary" onClick={() => setModal("account")}>
              Your account
            </button>
          )}
          {info?.auth_enabled && operator && (
            <button
              className="secondary"
              onClick={() => {
                endSession();
              }}
            >
              Lock this browser
            </button>
          )}
        </Modal>
      )}
      {modal === "team" && (
        <TeamDialog
          data={workspaces}
          operator={operator}
          session={info?.session}
          close={() => setModal(undefined)}
        />
      )}
      {modal === "account" && info?.session && (
        <AccountDialog
          session={info.session}
          close={() => setModal(undefined)}
          ended={endSession}
        />
      )}
    </div>
  );
}

// Read the one-use challenge once outside React StrictMode. Never leave it in
// browser history or send it to the server as a URL parameter.
const pairingCode = new URLSearchParams(location.hash.slice(1)).get("pair");
if (pairingCode)
  history.replaceState(null, "", location.pathname + location.search);
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {location.pathname === "/phone" ? (
      <PhoneApp pairingCode={pairingCode} />
    ) : (
      <App />
    )}
  </StrictMode>,
);

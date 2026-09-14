import { StrictMode, useEffect, useRef, useState, type FormEvent } from "react";
import { createRoot } from "react-dom/client";
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
import {
  WorkspacesDialog,
  type Workspaces,
} from "./components/WorkspacesDialog";

function App() {
  const [info, setInfo] = useState<Info>();
  const [locked, setLocked] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [selected, setSelected] = useState<string>();
  const [query, setQuery] = useState("");
  const [otpOnly, setOtpOnly] = useState(false);
  const [scenarios, setScenarios] = useState<Scenario[]>([]);
  const [composeMode, setComposeMode] = useState("capture");
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
  >();
  const [tab, setTab] = useState<"message" | "json" | "events">("message");
  const [busy, setBusy] = useState(false);
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem("textdock-theme") ||
      (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  );
  const search = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (info && !locked)
      void api<{ scenarios: Scenario[] }>(
        `/scenarios?inbox=${encodeURIComponent(inbox)}`,
      )
        .then((data) => setScenarios(data.scenarios))
        .catch((e) => setError(e.message));
  }, [info, locked, inbox, refresh]);
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
    setInbox(id);
    setFilters({ to: "", run_id: "", tag: "", since: "" });
    setQuery("");
    setFavorites(false);
    setOtpOnly(false);
    resetPage();
    setRefresh((n) => n + 1);
  }
  useEffect(() => {
    if (info && !locked)
      void api<Workspaces>("/workspaces")
        .then(setWorkspaces)
        .catch((e) => setError(e.message));
  }, [info, locked, refresh]);
  const live = useLiveEvents(
    info && !locked ? "/api/v1/events" : undefined,
    sessionStorage.getItem("textdock-token"),
  );
  useEffect(() => {
    if (live.state === "unauthorized") {
      setLocked(true);
      setMessages([]);
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
    try {
      setInfo(await api<Info>("/info"));
      setLocked(false);
      setError("");
    } catch (e) {
      if (e instanceof APIError && e.status === 401) setLocked(true);
      setError((e as Error).message);
    }
  }
  useEffect(() => {
    void loadInfo();
  }, []);
  useEffect(() => {
    if (!info || locked) return;
    let stopped = false;
    const controller = new AbortController();
    async function poll() {
      try {
        const data = await api<{ messages: Message[]; next_cursor: string }>(
          `/messages?${queryString}`,
          { signal: controller.signal },
        );
        if (!stopped) {
          setMessages(data.messages);
          setNextCursor(data.next_cursor);
          setError("");
        }
      } catch (e) {
        if (!stopped) {
          if (e instanceof APIError && e.status === 401) {
            setLocked(true);
            setMessages([]);
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
  }, [info, locked, queryString, refresh, live.revision]);

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
          }),
        },
      );
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
      setModal(undefined);
      setRefresh((n) => n + 1);
      setNotice(
        composeMode === "capture" ? "Message captured." : "Simulation started.",
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function remove(m: Message) {
    setBusy(true);
    try {
      await api(`/messages/${m.id}`, { method: "DELETE" });
      setSelected(undefined);
      setMessages((previous) => previous.filter((item) => item.id !== m.id));
      setRefresh((n) => n + 1);
      setNotice("Message deleted");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
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
    try {
      const updated = await api<Message>(`/messages/${m.id}`, {
        method: "PATCH",
        body: JSON.stringify(patch),
      });
      setMessages((items) =>
        items.map((item) => (item.id === updated.id ? updated : item)),
      );
      setRefresh((n) => n + 1);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function deleteChecked() {
    setBusy(true);
    try {
      await api("/messages/delete", {
        method: "POST",
        body: JSON.stringify({ ids: checked }),
      });
      resetPage();
      setRefresh((n) => n + 1);
      setNotice("Selected messages deleted.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function exportPage() {
    try {
      const token = sessionStorage.getItem("textdock-token");
      const response = await fetch(`/api/v1/export?${queryString}&format=csv`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!response.ok) throw new Error("Export failed");
      const url = URL.createObjectURL(await response.blob());
      const a = document.createElement("a");
      a.href = url;
      a.download = "textdock.csv";
      a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setError((e as Error).message);
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
        <p>Enter this server’s TextDock token to open the inbox.</p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            sessionStorage.setItem(
              "textdock-token",
              String(new FormData(e.currentTarget).get("token")),
            );
            void loadInfo();
          }}
        >
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
          <button className="primary">
            Unlock inbox <ArrowUpRight size={16} />
          </button>
        </form>
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
          <button className="nav-item" onClick={() => setModal("scenarios")}>
            <FlaskConical size={18} />
            Scenarios
          </button>
          <button className="nav-item" onClick={() => setModal("connect")}>
            <Smartphone size={18} />
            Open on phone
          </button>
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
            <span
              className={
                error || live.state !== "connected"
                  ? "live-dot offline"
                  : "live-dot"
              }
            />
            {error
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
            <p>Local messages · No real SMS delivery</p>
          </div>
          <button
            className="primary"
            onClick={() => {
              setError("");
              setModal("compose");
            }}
          >
            <Plus size={17} />
            Send test SMS
          </button>
        </section>
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
              disabled={busy}
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
                      disabled={busy}
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
                    <MessageEvents id={current.id} revision={live.revision} />
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
                        {current.mode === "simulate"
                          ? `Simulated ${current.direction}`
                          : "Captured locally"}{" "}
                        · No SMS sent
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
                          <dd>Not sent</dd>
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
                        <button className="secondary">Save tags</button>
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
                <option value="simulate">Simulate delivery</option>
                <option value="inbound">Simulate incoming SMS</option>
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
            {composeMode !== "capture" && (
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
                {busy ? "Capturing…" : "Capture message"}
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
      {modal === "workspaces" && (
        <WorkspacesDialog
          data={workspaces}
          select={chooseInbox}
          close={() => setModal(undefined)}
        />
      )}
      {modal === "commands" && (
        <Modal title="Commands" close={() => setModal(undefined)}>
          <div className="command-list">
            <button onClick={() => setModal("compose")}>Send test SMS</button>
            <button onClick={() => setModal("connect")}>Connect a phone</button>
            <button onClick={() => setModal("workspaces")}>
              Manage projects and inboxes
            </button>
            <button onClick={() => setModal("integrate")}>
              API integration
            </button>
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
          {info?.auth_enabled && (
            <button
              className="secondary"
              onClick={() => {
                sessionStorage.removeItem("textdock-token");
                setMessages([]);
                setInfo(undefined);
                setModal(undefined);
                setLocked(true);
              }}
            >
              Lock this browser
            </button>
          )}
        </Modal>
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

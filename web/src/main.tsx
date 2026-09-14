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
} from "lucide-react";
import { api, APIError, type Info, type Message } from "./api";
import "./styles.css";
import { Modal } from "./components/Modal";
import { ConnectDialog } from "./components/ConnectDialog";
import { PhoneApp } from "./PhoneApp";
import { useLiveEvents } from "./hooks/useLiveEvents";
import { copyText } from "./clipboard";

function App() {
  const [info, setInfo] = useState<Info>();
  const [locked, setLocked] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [selected, setSelected] = useState<string>();
  const [query, setQuery] = useState("");
  const [otpOnly, setOtpOnly] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [modal, setModal] = useState<
    "compose" | "connect" | "settings" | "integrate"
  >();
  const [tab, setTab] = useState<"message" | "json">("message");
  const [busy, setBusy] = useState(false);
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem("textdock-theme") ||
      (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  );
  const search = useRef<HTMLInputElement>(null);
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
        const data = await api<{ messages: Message[] }>(
          `/messages?q=${encodeURIComponent(query)}`,
          { signal: controller.signal },
        );
        if (!stopped) {
          setMessages(data.messages);
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
  }, [info, locked, query, refresh, live.revision]);

  const visible = messages.filter((m) => !otpOnly || m.analysis.otp);
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
      const m = await api<Message>("/messages", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(data)),
      });
      setQuery("");
      setOtpOnly(false);
      setMessages((previous) => [
        m,
        ...previous.filter((item) => item.id !== m.id),
      ]);
      setSelected(m.id);
      setModal(undefined);
      setRefresh((n) => n + 1);
      setNotice("Message captured. No real SMS sent.");
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

  const curl = [
    `curl ${location.origin}/api/v1/messages`,
    "  -H 'Content-Type: application/json'",
    ...(info?.auth_enabled
      ? ["  -H 'Authorization: Bearer YOUR_TEXTDOCK_TOKEN'"]
      : []),
    `  -d '{"to":"+33612345678","from":"Acme","body":"Your code is 482193","run_id":"signup-1"}'`,
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
            className={!otpOnly ? "nav-item active" : "nav-item"}
            onClick={() => {
              setOtpOnly(false);
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
              setSelected(undefined);
            }}
          >
            <KeyRound size={18} />
            OTP codes
          </button>
          <button className="nav-item" onClick={() => setModal("integrate")}>
            <Code2 size={18} />
            Integration
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
          <div className="breadcrumb">Local workspace</div>
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
              {otpOnly ? "OTP codes" : "Message inbox"}
              <span>{visible.length}</span>
            </h1>
            <p>Captured SMS · No real delivery</p>
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
                setSelected(undefined);
              }}
            />
            <kbd>/</kbd>
          </label>
          <span className="toolbar-note">
            <Radio size={14} />{" "}
            {live.state === "connected" ? "Live updates" : "Reconnecting…"}
          </span>
        </div>

        <div className={`inbox-layout ${current ? "has-selection" : ""}`}>
          <section className="message-list" aria-label="Messages">
            <div className="list-heading">
              <span>{otpOnly ? "With OTP" : "Latest messages"}</span>
              <span>{visible.length} shown · max 100</span>
            </div>
            {visible.length ? (
              visible.map((m) => (
                <button
                  key={m.id}
                  className={`message-row ${current?.id === m.id ? "selected" : ""}`}
                  onClick={() => {
                    setSelected(m.id);
                    setTab("message");
                  }}
                >
                  <span className="message-summary">
                    <span className="row-title">
                      <strong>{m.to}</strong>
                      <time>
                        {new Date(m.created_at).toLocaleTimeString([], {
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </time>
                    </span>
                    <span className="row-body">{m.body}</span>
                    <span className="row-tags">
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
                  <span className="status-pill">
                    <Check size={12} />
                    Captured
                  </span>
                </div>
                <div className="detail-content">
                  {tab === "json" ? (
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
                        Captured locally · No SMS sent
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
                      <dl className="metadata">
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
        <ConnectDialog close={() => setModal(undefined)} />
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
              <dd>Capture only</dd>
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

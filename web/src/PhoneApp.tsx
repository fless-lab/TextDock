import { useEffect, useState, type FormEvent } from "react";
import { Copy, LogOut, MessageSquare, Moon, Sun } from "lucide-react";
import { APIError, type Message } from "./api";
import type { Device } from "./components/ConnectDialog";
import { copyText } from "./clipboard";
import { useLiveEvents } from "./hooks/useLiveEvents";

const key = "textdock-device-token";
async function deviceAPI<T>(
  path: string,
  token: string | null,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(`/connect/v1${path}`, {
    ...options,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(options.body ? { "Content-Type": "application/json" } : {}),
    },
  });
  const data = await response.json();
  if (!response.ok)
    throw new APIError(response.status, data.error || "Request failed");
  return data;
}

export function PhoneApp({ pairingCode }: { pairingCode: string | null }) {
  const [token, setToken] = useState(() => localStorage.getItem(key));
  const [device, setDevice] = useState<Device>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [code, setCode] = useState(pairingCode);
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem("textdock-theme") ||
      (matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"),
  );
  const live = useLiveEvents(
    token && device ? "/connect/v1/events" : undefined,
    token,
  );

  function disconnect(reason: string) {
    localStorage.removeItem(key);
    setToken(null);
    setDevice(undefined);
    setMessages([]);
    setError(reason);
  }
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("textdock-theme", theme);
  }, [theme]);
  useEffect(() => {
    if (!token || code) return;
    const controller = new AbortController();
    void deviceAPI<Device>("/session", token, { signal: controller.signal })
      .then(setDevice)
      .catch((e) => {
        if (controller.signal.aborted) return;
        if (e instanceof APIError && e.status === 401)
          disconnect("This session has ended. Scan a new pairing code.");
        else setError(e.message);
      });
    return () => controller.abort();
  }, [token, code]);
  useEffect(() => {
    if (live.state === "unauthorized")
      disconnect("This session has ended. Scan a new pairing code.");
  }, [live.state]);
  useEffect(() => {
    if (!token || !device) return;
    const controller = new AbortController();
    void deviceAPI<{ messages: Message[] }>("/messages", token, {
      signal: controller.signal,
    })
      .then((data) => {
        setMessages(data.messages);
        setError("");
      })
      .catch((e) => {
        if (controller.signal.aborted) return;
        if (e instanceof APIError && e.status === 401)
          disconnect("This session has ended. Scan a new pairing code.");
        else setError(e.message);
      });
    return () => controller.abort();
  }, [token, device, live.revision]);
  useEffect(() => {
    if (notice) {
      const timer = setTimeout(() => setNotice(""), 3000);
      return () => clearTimeout(timer);
    }
  }, [notice]);
  async function claim(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await deviceAPI<{ token: string; device: Device }>(
        "/claim",
        null,
        {
          method: "POST",
          body: JSON.stringify({
            code,
            name: new FormData(e.currentTarget).get("name"),
          }),
        },
      );
      localStorage.setItem(key, result.token);
      setToken(result.token);
      setDevice(result.device);
      setCode(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="phone-shell">
      <header className="phone-header">
        <span className="brand">
          <MessageSquare size={21} />
          TextDock
        </span>
        <div>
          <button
            className="icon-button"
            aria-label="Toggle color theme"
            onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
          >
            {theme === "dark" ? <Sun size={18} /> : <Moon size={18} />}
          </button>
          {token && (
            <button
              className="icon-button"
              aria-label="Disconnect this browser"
              onClick={() =>
                disconnect("Disconnected. Scan a pairing code to reconnect.")
              }
            >
              <LogOut size={17} />
            </button>
          )}
        </div>
      </header>
      {code ? (
        <section className="phone-setup">
          <h1>Connect this phone</h1>
          <p className="modal-description">
            Choose a name to identify this device. This grants read-only access
            to the recipient selected on your computer.
          </p>
          <form
            onSubmit={(e) => {
              void claim(e);
            }}
          >
            <label>
              Device name
              <input
                name="name"
                defaultValue="My phone"
                maxLength={64}
                required
              />
            </label>
            <button className="primary" disabled={busy}>
              {busy ? "Connecting…" : "Connect phone"}
            </button>
          </form>
        </section>
      ) : device ? (
        <>
          <section className="phone-recipient">
            <h1>{device.scope.to}</h1>
            <p>
              {device.name} · Read only
              {device.scope.run_id ? ` · ${device.scope.run_id}` : ""}
            </p>
            <span className="connection">
              <span
                className={
                  live.state === "connected" ? "live-dot" : "live-dot offline"
                }
              />
              {live.state === "connected" ? "Live" : "Reconnecting…"}
            </span>
          </section>
          <section className="phone-messages" aria-label="Phone messages">
            {messages.length ? (
              [...messages].reverse().map((message) => (
                <article key={message.id} className="phone-message">
                  <header>
                    <strong>{message.from}</strong>
                    <time>
                      {new Date(message.created_at).toLocaleString([], {
                        dateStyle: "short",
                        timeStyle: "short",
                      })}
                    </time>
                  </header>
                  <p>{message.body}</p>
                  {message.analysis.otp && (
                    <div className="phone-code">
                      <strong>{message.analysis.otp}</strong>
                      <button
                        className="secondary"
                        onClick={async () =>
                          setNotice(
                            (await copyText(message.analysis.otp!))
                              ? "Code copied."
                              : "Select the code to copy it.",
                          )
                        }
                      >
                        <Copy size={14} />
                        Copy code
                      </button>
                    </div>
                  )}
                </article>
              ))
            ) : (
              <p className="phone-empty">
                Waiting for messages to this number.
              </p>
            )}
          </section>
          <p className="phone-note">
            Local inbox · Latest 100 messages · Keep this page open for live
            updates.
          </p>
        </>
      ) : (
        !code && (
          <section className="phone-setup">
            <h1>{token ? "Connecting…" : "Phone inbox"}</h1>
            <p className="modal-description">
              Open “Connect a phone” on your computer and scan its pairing QR
              code.
            </p>
          </section>
        )
      )}
      {error && (
        <p role="alert" className="error-text phone-error">
          {error}
        </p>
      )}
      {notice && (
        <div className="toast" role="status">
          {notice}
        </div>
      )}
    </main>
  );
}

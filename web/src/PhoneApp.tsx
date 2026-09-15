import { useEffect, useState, type FormEvent } from "react";
import { Copy, LogOut, MessageSquare, Moon, Sun } from "lucide-react";
import { APIError, type Message } from "./api";
import type { Device } from "./components/ConnectDialog";
import { copyText } from "./clipboard";
import { useLiveEvents } from "./hooks/useLiveEvents";
import { deviceAPI } from "./phone/api";
import { disableNotifications } from "./phone/push";
import { PhoneNotifications } from "./components/PhoneNotifications";

const key = "textdock-device-token";

export function PhoneApp({ pairingCode }: { pairingCode: string | null }) {
  const [token, setToken] = useState(() => localStorage.getItem(key));
  const [device, setDevice] = useState<Device>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [code, setCode] = useState(pairingCode);
  const [manualCode, setManualCode] = useState(pairingCode || "");
  const [refresh, setRefresh] = useState(0);
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
    void disableNotifications(token).catch(() => {});
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
    if (!("serviceWorker" in navigator)) return;
    const update = (event: MessageEvent) => {
      if (event.data?.type === "textdock-refresh") setRefresh((n) => n + 1);
    };
    navigator.serviceWorker.addEventListener("message", update);
    return () => navigator.serviceWorker.removeEventListener("message", update);
  }, []);
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
  }, [token, device, live.revision, refresh]);
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
      let challenge = manualCode.trim();
      if (challenge.includes("://")) {
        const url = new URL(challenge);
        if (url.origin !== location.origin)
          throw new Error(
            "This pairing link belongs to another TextDock server.",
          );
        challenge = new URLSearchParams(url.hash.slice(1)).get("pair") || "";
      }
      if (!challenge)
        throw new Error("Paste a pairing code or link from the desktop.");
      const result = await deviceAPI<{ token: string; device: Device }>(
        "/claim",
        null,
        {
          method: "POST",
          body: JSON.stringify({
            code: challenge,
            name: new FormData(e.currentTarget).get("name"),
          }),
        },
      );
      await disableNotifications(token).catch(() => {});
      localStorage.setItem(key, result.token);
      setToken(result.token);
      setDevice(result.device);
      setCode(null);
      setManualCode("");
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
      {code || !token ? (
        <section className="phone-setup">
          <h1>Connect this phone</h1>
          <p className="modal-description">
            Choose a name to identify this device. This grants read-only access
            to the recipient selected on your computer.
          </p>
          <p className="modal-description">
            For home-screen use on iPhone/iPad, install this page from Safari,
            open the installed app, then paste a fresh pairing link here.
          </p>
          <form
            onSubmit={(e) => {
              void claim(e);
            }}
          >
            <label>
              Pairing code or link
              <input
                value={manualCode}
                onChange={(e) => setManualCode(e.target.value)}
                autoComplete="off"
                autoCapitalize="none"
                spellCheck={false}
                required
                placeholder="Paste the code or link from TextDock"
              />
            </label>
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
          {token && (
            <PhoneNotifications
              token={token}
              deviceId={device.id}
              expiresAt={device.expires_at}
            />
          )}
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

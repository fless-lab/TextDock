import { useEffect, useRef, useState } from "react";
import { Bell, Download, RefreshCw } from "lucide-react";
import { deviceAPI } from "../phone/api";
import {
  bindNotifications,
  disableNotifications,
  phoneWorker,
  publicKeyBytes,
  samePublicKey,
} from "../phone/push";

interface Configuration {
  enabled: boolean;
  public_key: string;
}
interface State {
  subscribed: boolean;
  generation?: string;
  endpoint_host?: string;
  status: string;
  http_status: number;
  detail?: string;
  updated_at?: string;
  queued: boolean;
}
interface InstallEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: string }>;
}
const ios = () =>
  /iPad|iPhone|iPod/.test(navigator.userAgent) ||
  (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
const standalone = () =>
  matchMedia("(display-mode: standalone)").matches ||
  (navigator as Navigator & { standalone?: boolean }).standalone === true;

export function PhoneNotifications({
  token,
  deviceId,
  expiresAt,
}: {
  token: string;
  deviceId: string;
  expiresAt: string;
}) {
  const lifetime = useRef(new AbortController());
  const revision = useRef(0);
  const mutating = useRef(false);
  const [config, setConfig] = useState<Configuration>();
  const [state, setState] = useState<State>();
  const [reg, setReg] = useState<ServiceWorkerRegistration>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [permission, setPermission] = useState(() =>
    "Notification" in window ? Notification.permission : "unsupported",
  );
  const [install, setInstall] = useState<InstallEvent>();
  const [installed, setInstalled] = useState(standalone);
  const [registered, setRegistered] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const secure = window.isSecureContext;
  const supported =
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window;

  useEffect(() => {
    const prompt = (event: Event) => {
      event.preventDefault();
      setInstall(event as InstallEvent);
    };
    const done = () => {
      setInstalled(true);
      setInstall(undefined);
    };
    window.addEventListener("beforeinstallprompt", prompt);
    window.addEventListener("appinstalled", done);
    return () => {
      window.removeEventListener("beforeinstallprompt", prompt);
      window.removeEventListener("appinstalled", done);
    };
  }, []);
  useEffect(() => {
    let stopped = false;
    if (secure && "serviceWorker" in navigator)
      void phoneWorker()
        .then((value) => {
          if (!stopped) setReg(value);
        })
        .catch((e) => {
          if (!stopped) setError(e.message);
        });
    return () => {
      stopped = true;
    };
  }, [secure]);

  async function refresh() {
    if (mutating.current) return;
    const signal = lifetime.current.signal;
    const sequence = revision.current;
    try {
      const [configuration, current] = await Promise.all([
        deviceAPI<Configuration>("/push/config", token, { signal }),
        deviceAPI<State>("/push/state", token, { signal }),
      ]);
      if (signal.aborted || sequence !== revision.current) return;
      setConfig(configuration);
      setState(current);
      const allowed =
        "Notification" in window ? Notification.permission : "unsupported";
      setPermission(allowed);
      if (!reg || !supported) {
        setRegistered(false);
        return;
      }
      const subscription = await reg.pushManager.getSubscription();
      if (signal.aborted || sequence !== revision.current) return;
      const matches =
        !!subscription &&
        !!configuration.public_key &&
        samePublicKey(
          subscription.options.applicationServerKey,
          publicKeyBytes(configuration.public_key),
        );
      const active =
        configuration.enabled &&
        current.subscribed &&
        allowed === "granted" &&
        matches;
      setRegistered(active);
      await bindNotifications(
        reg,
        active ? deviceId : "",
        active ? current.generation || "" : "",
        expiresAt,
      );
      if (signal.aborted || sequence !== revision.current) return;
      if (current.subscribed && allowed === "denied")
        await disableNotifications(token);
    } catch (e) {
      if (!signal.aborted && sequence === revision.current)
        setError((e as Error).message);
    }
  }
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    void refresh();
    const focus = () => {
      if (!document.hidden) void refresh();
    };
    document.addEventListener("visibilitychange", focus);
    return () => {
      controller.abort();
      revision.current++;
      document.removeEventListener("visibilitychange", focus);
    };
  }, [token, deviceId, expiresAt, reg]);
  useEffect(() => {
    if (!expanded || !config?.enabled) return;
    const timer = setInterval(() => {
      if (!document.hidden) void refresh();
    }, 10000);
    return () => clearInterval(timer);
  }, [expanded, config?.enabled, token, deviceId, reg]);

  async function enable() {
    mutating.current = true;
    revision.current++;
    const signal = lifetime.current.signal;
    setBusy(true);
    setError("");
    try {
      // Permission is requested only from this explicit user action.
      const result =
        Notification.permission === "granted"
          ? "granted"
          : await Notification.requestPermission();
      if (signal.aborted) return;
      setPermission(result);
      if (result !== "granted")
        throw new Error(
          "Notifications were not allowed. Change this site’s browser permission to enable them.",
        );
      if (!reg || !config?.enabled)
        throw new Error("Notifications are not ready on this server");
      const key = publicKeyBytes(config.public_key);
      let sub = await reg.pushManager.getSubscription();
      if (
        sub &&
        (!samePublicKey(sub.options.applicationServerKey, key) ||
          state?.status === "expired")
      ) {
        await sub.unsubscribe();
        sub = null;
      }
      sub ||= await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: key,
      });
      if (signal.aborted) return;
      const current = await deviceAPI<State>("/push/subscription", token, {
        method: "PUT",
        body: JSON.stringify(sub.toJSON()),
        signal,
      });
      if (signal.aborted) return;
      await bindNotifications(
        reg,
        deviceId,
        current.generation || "",
        expiresAt,
      );
      setState(current);
      setRegistered(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      mutating.current = false;
      setBusy(false);
    }
  }
  async function disable() {
    mutating.current = true;
    revision.current++;
    setBusy(true);
    setError("");
    try {
      await disableNotifications(token);
      setRegistered(false);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      mutating.current = false;
      setBusy(false);
      void refresh();
    }
  }
  async function test() {
    setBusy(true);
    setError("");
    try {
      await deviceAPI("/push/test", token, { method: "POST" });
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <details
      className="phone-notifications"
      onToggle={(event) => setExpanded(event.currentTarget.open)}
    >
      <summary>
        <Bell size={15} />
        Notifications & installation
      </summary>
      {!installed && (
        <div className="phone-install">
          <h3>Install the phone inbox</h3>
          {install ? (
            <button
              className="secondary"
              onClick={async () => {
                await install.prompt();
                await install.userChoice;
                setInstall(undefined);
              }}
            >
              <Download size={14} />
              Install TextDock
            </button>
          ) : (
            <p>
              Use your browser’s “Add to Home Screen” option. On iPhone/iPad,
              install from Safari, then open the home-screen app and pair it
              there.
            </p>
          )}
        </div>
      )}
      <h3>Message notifications</h3>
      <p>
        Alerts contain no SMS text, number or OTP. Tap an alert to open this
        paired inbox.
      </p>
      {!secure ? (
        <p className="callout">
          Notifications and installation require a trusted HTTPS connection.
          Plain HTTP on a LAN address does not provide a secure context.
        </p>
      ) : ios() && !installed ? (
        <p className="callout">
          On iPhone/iPad, open the installed home-screen app before enabling Web
          Push.
        </p>
      ) : !supported ? (
        <p className="callout">
          This browser does not provide Web Push. The inbox can still update
          while open.
        </p>
      ) : !config?.enabled ? (
        <p className="callout">
          Push is disabled on the server. Configure TEXTDOCK_PUSH_ENABLED,
          TEXTDOCK_PUBLIC_URL and a VAPID contact to enable it.
        </p>
      ) : (
        <>
          <p>
            Browser permission: <strong>{permission}</strong> · Session:{" "}
            <strong>
              {registered ? "notifications enabled" : "notifications off"}
            </strong>
          </p>
          <div className="phone-push-actions">
            {registered ? (
              <>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => {
                    void disable();
                  }}
                >
                  Disable notifications
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => {
                    void test();
                  }}
                >
                  Send test notification
                </button>
              </>
            ) : (
              <button
                className="primary"
                disabled={busy || !reg || permission === "denied"}
                onClick={() => {
                  void enable();
                }}
              >
                Enable notifications
              </button>
            )}
            <button
              className="icon-button"
              aria-label="Refresh notification status"
              disabled={busy}
              onClick={() => {
                setError("");
                void refresh();
              }}
            >
              <RefreshCw size={15} />
            </button>
          </div>
          {permission === "denied" && (
            <p>
              Use the browser’s site settings to change the blocked permission,
              then refresh this status.
            </p>
          )}
        </>
      )}
      {state?.detail && (
        <p className="push-diagnostic">
          {state.detail}
          {state.http_status ? ` (HTTP ${state.http_status})` : ""}
          {state.queued ? " · Alert queued" : ""}
        </p>
      )}
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
      <p>
        Background delivery depends on the browser/platform push service and
        Internet connectivity. Returning to the local inbox may still require
        your Wi-Fi or VPN.
      </p>
    </details>
  );
}

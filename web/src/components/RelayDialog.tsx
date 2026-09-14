import { useEffect, useState, type FormEvent } from "react";
import { api } from "../api";
import { copyText } from "../clipboard";
import { Modal } from "./Modal";
import { OTPFormat } from "./OTPFormat";

export interface RelayInfo {
  enabled: boolean;
  driver: string;
  limit_per_minute: number;
  queue_ttl_seconds?: number;
  allowed_to?: string[];
  receipts_enabled?: boolean;
}
interface Gateway {
  id: string;
  name: string;
  expires_at: string;
  revoked: boolean;
  online?: boolean;
  last_seen?: string;
  model?: string;
  app_version?: string;
  subscription_id?: number;
}
export function RelayDialog({
  info,
  close,
  version = "",
}: {
  info: RelayInfo;
  close: () => void;
  version?: string;
}) {
  const apkURL = /^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/.test(version)
    ? `https://github.com/fless-lab/TextDock/releases/download/${version}/textdock-gateway-${version}.apk`
    : "https://github.com/fless-lab/TextDock/releases";
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [token, setToken] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  async function refresh() {
    try {
      setGateways((await api<{ gateways: Gateway[] }>("/gateways")).gateways);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  useEffect(() => {
    if (info.enabled) void refresh();
    const timer = setInterval(() => {
      if (info.enabled && !document.hidden) void refresh();
    }, 15000);
    return () => clearInterval(timer);
  }, [info.enabled]);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      const result = await api<{ token: string }>("/gateways", {
        method: "POST",
        body: JSON.stringify({
          name: new FormData(e.currentTarget).get("name"),
        }),
      });
      setToken(result.token);
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function revoke(id: string) {
    try {
      await api(`/gateways/${id}`, { method: "DELETE" });
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <Modal title="Real SMS relay" close={close}>
      <p className="modal-description">
        Relay mode sends real SMS through the configured provider or an enrolled
        Android phone. Capture and simulation remain local.
      </p>
      <dl className="metadata">
        <div>
          <dt>Driver</dt>
          <dd>{info.driver || "Disabled"}</dd>
        </div>
        <div>
          <dt>Sending limit</dt>
          <dd>{info.limit_per_minute} messages/minute</dd>
        </div>
        {info.enabled && (
          <div>
            <dt>Queue expiration</dt>
            <dd>{info.queue_ttl_seconds}s</dd>
          </div>
        )}
        <div>
          <dt>Allowed recipients</dt>
          <dd>
            {info.allowed_to?.length
              ? info.allowed_to.join(", ")
              : "Any explicitly addressed recipient"}
          </dd>
        </div>
      </dl>
      {!info.enabled && (
        <>
          <h3>Enable on the server</h3>
          <pre className="code-block">{`# Twilio\nTEXTDOCK_RELAY_DRIVER=twilio\nTWILIO_ACCOUNT_SID=...\nTWILIO_AUTH_TOKEN=...\n\n# Android SIM gateway\nTEXTDOCK_RELAY_DRIVER=android`}</pre>
          <p className="modal-description">
            Restart TextDock with these environment settings. Optional:
            TEXTDOCK_RELAY_ALLOWED_TO, TEXTDOCK_RELAY_LIMIT and
            TEXTDOCK_RELAY_TTL.
          </p>
        </>
      )}
      {info.driver === "twilio" && (
        <p className="modal-description">
          Use an authorized Twilio sender.{" "}
          {info.receipts_enabled
            ? "Signed delivery receipts are enabled."
            : "Set TEXTDOCK_PUBLIC_URL to a reachable origin to receive signed delivery receipts."}{" "}
          Uncertain sends are never retried automatically.
        </p>
      )}
      {info.driver === "android" && (
        <>
          <h3>Enroll an Android gateway</h3>
          <a
            className="secondary"
            href={apkURL}
            target="_blank"
            rel="noreferrer"
          >
            Download Android gateway APK
          </a>
          <p className="modal-description">
            Install the development APK from the release, then enter this
            server’s LAN/HTTPS address and the gateway token in the app. The
            phone uses its default SIM; sent means submission to the mobile
            network, not confirmed delivery.
          </p>
          <form
            onSubmit={(e) => {
              void create(e);
            }}
          >
            <label>
              Gateway name
              <input
                name="name"
                maxLength={80}
                required
                placeholder="e.g. Test Android"
              />
            </label>
            <button className="primary">Create gateway token</button>
          </form>
          {token && (
            <div className="pairing-result">
              <input
                aria-label="Gateway token"
                value={token}
                readOnly
                onFocus={(e) => e.currentTarget.select()}
              />
              <button
                className="secondary"
                onClick={async () =>
                  setNotice(
                    (await copyText(token))
                      ? "Gateway token copied."
                      : "Select the token to copy it.",
                  )
                }
              >
                Copy token
              </button>
              <p>Save it now. The server stores only its hash.</p>
            </div>
          )}
          <ul className="device-list">
            {gateways.map((g) => (
              <li key={g.id}>
                <div>
                  <strong>{g.name}</strong>
                  <small>
                    {g.revoked
                      ? "Revoked"
                      : g.online
                        ? "Online"
                        : g.last_seen
                          ? "Offline"
                          : "No heartbeat yet"}
                    {g.model ? ` · ${g.model}` : ""}
                    {g.app_version ? ` · ${g.app_version}` : ""}
                  </small>
                  {g.last_seen && (
                    <small>
                      Last seen {new Date(g.last_seen).toLocaleString()} ·{" "}
                      {g.subscription_id === -1
                        ? "Default SIM"
                        : `SIM subscription ${g.subscription_id}`}
                    </small>
                  )}
                  <small>{new Date(g.expires_at).toLocaleDateString()}</small>
                </div>
                {g.revoked ? (
                  <span className="muted">Revoked</span>
                ) : (
                  <button
                    className="text-button"
                    onClick={() => {
                      void revoke(g.id);
                    }}
                  >
                    Revoke
                  </button>
                )}
              </li>
            ))}
          </ul>
          <button
            className="secondary"
            onClick={() => {
              void refresh();
            }}
          >
            Refresh gateway status
          </button>
        </>
      )}
      <OTPFormat />
      {notice && <p role="status">{notice}</p>}
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
    </Modal>
  );
}

export function RelayDetails({
  id,
  revision,
}: {
  id: string;
  revision: number;
}) {
  const [job, setJob] = useState<{
    driver: string;
    state: string;
    provider_id: string;
    owner: string;
    error: string;
  }>();
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    void api<typeof job>(`/messages/${id}/relay`, { signal: controller.signal })
      .then(setJob)
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, [id, revision]);
  return (
    <section>
      <h3>Real SMS dispatch</h3>
      {job && (
        <dl className="metadata">
          <div>
            <dt>Transport</dt>
            <dd>{job.driver}</dd>
          </div>
          <div>
            <dt>State</dt>
            <dd>{job.state}</dd>
          </div>
          <div>
            <dt>Provider reference</dt>
            <dd>{job.provider_id || "Not available"}</dd>
          </div>
          {job.owner && (
            <div>
              <dt>Worker / gateway</dt>
              <dd>{job.owner}</dd>
            </div>
          )}
          {job.error && (
            <div>
              <dt>Detail</dt>
              <dd>{job.error}</dd>
            </div>
          )}
        </dl>
      )}
      {job?.state === "unknown" && (
        <p className="modal-description">
          Acceptance is uncertain. Inspect the provider or gateway before
          creating another send. TextDock will not resend this job
          automatically.
        </p>
      )}
      {error && <p className="error-text">{error}</p>}
    </section>
  );
}
